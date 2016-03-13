# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import signal
import socket
import time
import weakref

from threading import Thread

from .config import Configuration, LoggerWrapper
from .common import CommandBuilder
from .processing import CommandExecuter, WorkerStatusEnum


class ManageAdapter(object):
    service_methods = {}
    _cid = None

    def __init__(self, **methods):
        for name, m in methods.items():
            if callable(m):
                self.service_methods[name] = weakref.WeakMethod(m)

    def _external_method_call(self, method):
        result = None
        external_method_ref = self.service_methods.get(method)
        if external_method_ref:
            result = external_method_ref()
        return result

    def get_public_methods(self):
        return self._external_method_call('public_methods')

    def has_cid(self):
        return not(self._cid is None)

    def get_cid(self):
        return self._cid

    def setup_cid(self, value):
        self._cid = value

    def get_status(self):
        check_free = self._external_method_call('has_free_workers')
        result = WorkerStatusEnum.busy
        if check_free:
            result = WorkerStatusEnum.free

        return result


class ConnectionThread(Thread):
    _server = None
    _logger = None
    _conf = None
    _lock = None
    _connection = None
    _update_status_method = None

    cls_command_builder = CommandBuilder
    cls_command_executer = CommandExecuter

    def __init__(self, server, conf):
        super().__init__(name='roolt-connection')
        self._server = weakref.ref(server)
        self._conf = dict(**conf)
        self._logger = conf.get_logger(wraper=LoggerWrapper('connection'))

    def _connection_access(self, data=None, write=False):
        result = None
        if self._connection:
            while self._lock:
                time.sleep(0.1)

            self._lock = True
            try:
                if write:
                    self._connection.send(data)
                else:
                    read_buffer = self._conf.get('read_buffer') or 1024
                    result = self._connection.recv(read_buffer)
            finally:
                self._lock = False
        return result

    def flush(self):
        pass

    def run(self):
        self._lock = False
        server_active = True
        logger = self._logger
        retry_time = self._conf.get('retry_time') or 2.5
        encoding = self._conf.get('encoding') or 'utf-8'
        # connect
        first_run = True
        has_connection = False
        manager = ManageAdapter()
        command_builder = self.cls_command_builder()
        command_executer = self.cls_command_executer()
        command_executer.setup(
            configuration=self._conf,
            manager=manager,
            encoding=encoding)

        while server_active:

            # create connection or reconnect
            while not has_connection:
                if not first_run:
                    logger.warn('try reconnect after {}'.format(retry_time))
                    time.sleep(retry_time)

                try:
                    self._connection = socket.socket(
                        socket.AF_INET, socket.SOCK_STREAM)
                    self._connection.connect((
                        self._conf.get('addr'),
                        int(self._conf.get('port')),
                    ))
                except Exception as err:
                    logger.error(err)
                    has_connection = False
                else:
                    has_connection = True
                    logger.info('Connected to {}:{}'.format(
                        self._conf.get('addr'), int(self._conf.get('port'))))
                finally:
                    server = self._server()
                    server_active = server and server.is_alive()

                    if not server_active:
                        has_connection = False

                    first_run = False

            if has_connection and not manager.has_cid():
                # send hello
                answer = command_executer.get_hello()
                try:
                    self._connection_access(write=True, data=bytes(
                        '{}\n'.format(answer), encoding))

                except Exception as err:
                    logger.error(err)
                    has_connection = False

            if has_connection:
                # data change
                new_data = True
                while new_data and server_active:
                    try:
                        new_data = self._connection_access()
                    except Exception as err:
                        logger.error(err)
                        has_connection = False
                        new_data = None

                    if new_data:
                        new_data = new_data.decode(encoding)
                        new_data = new_data.split('\n')

                        for line_data in new_data:
                            command_builder.append(line_data)
                            if command_builder.is_done():
                                try:
                                    command = command_builder.get_command()
                                except ValueError as err:
                                    logger.error(err)
                                    answer = None
                                else:
                                    try:
                                        answer = command_executer.run(command)
                                    except Exception as err:
                                        logger.error(err)
                                        answer = command_executer.get_error(err)

                                if answer:
                                    logger.debug(answer)
                                    try:
                                        self._connection_access(
                                            write=True,
                                            data=bytes(
                                                '{}\n'.format(answer), encoding))

                                    except Exception as err:
                                        logger.error(err)
                                        logger.warn('Lost command: {}'.format(
                                            command))
                                        new_data = None
                                        has_connection = False

                    # if lost connection
                    if not has_connection:
                        self._connection = None

                    server = self._server()
                    server_active = server and server.is_alive()
                    # send close

        server = self._server()
        if self:
            server.close()


class Server(object):

    _conf = None
    _active = True
    _can_exit = False
    _connection = None

    def __init__(self, conf=None):
        if not conf:
            conf = Configuration()

        self._conf = conf
        self._connection = ConnectionThread(self, conf)
        self._logger = conf.get_logger(wraper=LoggerWrapper('server'))
        signal.signal(signal.SIGTERM, self._stop_sig_handler)
        signal.signal(signal.SIGINT, self._stop_sig_handler)

    def _stop_sig_handler(self, *args, **kwargs):
        self.stop()

    def stop(self):
        if self._active:
            self._logger.info('stoping')
            self._active = False

    def is_alive(self):
        return bool(self._active)

    def run(self):
        self._active = True
        self._can_exit = False
        self._logger.info('Server started...')
        self._connection.start()

        while self._active:
            time.sleep(self._conf.iter_time)
            self._connection.flush()

        not_first = False
        while not self._can_exit:
            time.sleep(1)
            if not_first:
                self._logger.warn('wait process finished')
            not_first = True

    def close(self):
        self._can_exit = True
