# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import asyncio
import weakref
import signal

from collections import defaultdict
from copy import copy
from functools import wraps

from .client import Connection, ServerError, ProtocolError
from .common import (
    Configuration, MetaOnceObject, get_object_path)
from .enums import GroupConnectionEnum
from .transport import Command, Answer, UnitBuilder, encoding


class NotReadyError(Exception):
    pass


class MethodRegistry(metaclass=MetaOnceObject):

    __methods = None
    # {method_path: method}
    __run_options = None
    # method standard options:
    # {
    #     'timeout': None,
    #     'progress': True,
    #     'logger': False,
    # }
    _default_run_options = {
        'timeout': None,
        'progress': True,
        'logger': True,
    }

    def __init__(self):
        self.__methods = {}
        self.__run_options = defaultdict(dict)

    @classmethod
    def is_empty(cls):
        reg = cls()

        return not any(
            callable(reg.get(method)[0]) for method in reg.methods_list)

    def options_update(self, method, **options):
        if callable(method) or isinstance(method, str) and method:
            if not isinstance(method, str):
                method = get_object_path(method)
            self.__run_options[method].update(**options)

    def setup(self, method, **options):
        self.set(method, method, **options)

    def set(self, target_method, call_method, **options):
        if callable(target_method) and callable(call_method):
            method_path = get_object_path(target_method)
            self.__methods[method_path] = call_method
            if options:
                self.options_update(method_path, **options)
        else:
            raise TypeError('Expected a any callable object.')

    def remove(self, method):
        if callable(method) or isinstance(method, str) and method:
            if not isinstance(method, str):
                method = get_object_path(method)
            if method in self.__methods:
                del self.__methods[method]

    def get(self, method):
        if callable(method) or isinstance(method, str) and method:
            if not isinstance(method, str):
                method = get_object_path(method)

        options = copy(self._default_run_options)
        options.update(self.__run_options.get(method) or {})
        method = self.__methods.get(method)
        if not callable(method):
            method = None
        return method, options

    @property
    def methods_list(self):
        return list(self.__methods.keys())


def rpc_method_wraper(**options):
    """
    Decorator for server RPC methods.
    Example:

    @rpc_method_wraper(logger=True)
    def rpc_method_sum(x, y, **options):
        logger = options.get('logger')
        result = x + y
        logger.debug('sum: {}'.format(result))
        return result

    Progress bar support for a long time processing.
    Useful where used web-socket client, and as well the opportunity
    call native method 'state' from
    simple client in loop (implemented polling).

    @rpc_method_wraper(progress=True, logger=True)
    def rpc_method_processing(params, **options):
        logger = options.get('logger')
        progress_bar = options.get('progress')

        operation_count = params.n
        progress_bar.total(operation_count)

        for _ in range(opertaion_count):
            ...do some thing
            progress_bar.step()

    Process execute time limiting support.

    @rpc_method_wraper(timeout=1800)
    def rpc_method_processing(params, **options):
        timer = options.get('timer')
        timer.set_message('method extend time limit 30 min')

        operation_count = params.n

        for _ in range(opertaion_count):
            ...do some thing
            timer.rasie_if_exceeded()

    """

    def method_decor(real_method):
        registry = MethodRegistry()

        @wraps(real_method)
        def method_wraper(*args, **kwargs):
            # update kwargs
            _, run_options = registry.get(real_method)

            method_kwargs = {
                key: value
                for key, value in kwargs if key not in ('server', 'task_id')
            }

            task_id = kwargs.get('task_id')
            progress_bar = timeout = None

            if task_id:
                # server proxy
                server = kwargs.get('server')

                if run_options.get('logger'):
                    method_kwargs['logger'] = server.get_logger()

                if run_options.get('progress'):
                    progress_bar = server.get_progress_bar(task_id)
                    method_kwargs['progress_bar'] = progress_bar

                timeout = run_options.get('timeout')

                if timeout:
                    timer = server.get_timer(timeout)
                    method_kwargs['timer'] = timer

            try:
                result = real_method(*args, **method_kwargs)
            finally:
                if progress_bar:
                    progress_bar.done()

                if timeout:
                    timer.stop()

            return result

        # registarion metod
        registry.set(real_method, method_wraper, **options)

    return method_decor


class Server(object):
    # multiprocessing group size
    _worker_pool_size = 2
    _income_queue = None
    _outcome_queue = None
    _conf = None
    _close_loop_after = True
    _event_loop = None
    _state = None
    _conn_reader = _conn_writer = None
    _buffer_size = 1024
    _reconnect_time = 2.5

    _handlers = {}

    __tmp = {}

    def __init__(
            self,
            event_loop=None,
            close_loop_after=True,
            unit_builder_cls=UnitBuilder,
            **options):
        """
        """

        if MethodRegistry.is_empty():
            raise NotReadyError(
                'No one public method. Server must have methods.')

        if event_loop is None:
            event_loop = asyncio.get_event_loop()
            close_loop_after = True

        if options:
            conf = Configuration(env_var=None, **options)
        else:
            conf = Configuration()

        self.__tmp['auth_data'] = Connection.prepare_auth_token(conf)
        self._conf = conf
        self._event_loop = event_loop
        self._close_loop_after = close_loop_after
        self.answer_builder = unit_builder_cls(Answer)
        # crypto data

    def is_connected(self):
        return bool(self._state.get('connected'))

    def close(self):
        if self._close_loop_after:
            self._event_loop.close()

    def connect(self):
        conf = self._conf
        logger = conf.get_logger()
        has_connect = False
        logger.info('Connected to {addr}:{port}'.format(**conf.as_dict()))
        while not has_connect:
            try:
                result = yield from asyncio.open_connection(
                    conf.get('addr'),
                    int(conf.get('port')),
                    loop=self._event_loop)
            except Exception as err:
                logger.error(err)
                has_connect = not self._state.get('active')
                self._state['connected'] = False
                yield from asyncio.sleep(self._reconnect_time)
            else:
                has_connect = True
                self._state['connected'] = True
                logger.info('connection establishment')
                if 'auth_data' not in self.__tmp:
                    self.__tmp['auth_data'] = Connection.prepare_auth_token(conf)

                return result

    def _init_server(self, conf, logger, reader, writer):
        result = False
        cmd = Command(
            method='auth',
            _data=self.__tmp.pop('auth_data'),
            json={'key': conf.get('crypto_pub_key_name')})

        writer.write(cmd.as_data().encode(encoding=encoding))
        answer_builder = self.answer_builder
        while not answer_builder.is_done():
            data = yield from reader.read(self._buffer_size)
            if data:
                answer_builder.append(data.decode(encoding))

        answer = answer_builder.get_unit()
        if answer and answer.has_error():
            err = answer.error
            serv_err = ServerError(**err)
            logger.fatal(serv_err)
        else:
            answer_data = answer.result_as_json()
            if isinstance(answer_data, dict) and 'auth' in answer_data:
                result = bool(answer_data.get('auth'))
            else:
                logger.error(
                    'Auth answer data has unknown format: {}.'.format(
                        answer.result))

        if result:
            result = False
            client_info = {
                'group': GroupConnectionEnum.Server.value,
                'methods': MethodRegistry().methods_list,
            }
            logger.info(
                'Public methods: {}'.format(
                    ', '.join(map('"{}"'.format, client_info['methods']))))

            cmd = Command(
                method='registration',
                # if cid still exists
                cid=self._state.get('cid'),
                json=client_info)

            writer.write(cmd.as_data().encode(encoding=encoding))
            while not answer_builder.is_done():
                data = yield from reader.read(self._buffer_size)
                if data:
                    answer_builder.append(data.decode(encoding))

            answer = answer_builder.get_unit()
            if answer and answer.has_error():
                err = answer.error
                serv_err = ServerError(**err)
                logger.fatal(serv_err)
            else:
                answer_data = answer.result_as_json() or {}
                if answer_data.get('ok'):
                    result = True
                    self._state['cid'] = answer_data.get('cid')
                else:
                    logger.error(
                        'Registration answer format wrong, data: {}'.format(
                            answer.result))

        return result

    def run(self):
        iter_delay = 0.1

        @asyncio.coroutine
        def manage_worker(server):
            logger = server._conf.get_logger()
            logger.info('Wait command')
            while server._state['active']:
                yield from asyncio.sleep(iter_delay)

            logger.info('Server stopping...')

        self._state = server_state = {'active': True}

        def stop_sig_handler(*args, **kwargs):
            server_state['active'] = False

        signal.signal(signal.SIGTERM, stop_sig_handler)
        signal.signal(signal.SIGINT, stop_sig_handler)

        @asyncio.coroutine
        def connection_service_worker(server):
            assert isinstance(server, Server)
            conf = self._conf
            logger = conf.get_logger()
            logger.info('Up connection..')

            while server._state['active']:
                try:
                    reader, writer = yield from server.connect()
                except TypeError:
                    yield from asyncio.sleep(iter_delay)
                else:
                    init_done = yield from server._init_server(
                        conf, logger, reader, writer)

                    if init_done:
                        logger.info('Authorization completed.')
                        # processing loop
                        while server._state['active']:
                            # TODO: wait output Queue
                            # TODO: wait command to input Queue
                            # TODO: get exception of broken connection
                            # and goto prev yield for new reader, writer
                            yield from asyncio.sleep(1)

                    else:
                        server._state['active'] = False
                        logger.error(
                            'Authorization answer incorrect.')

        loop = self._event_loop
        workers = [
            asyncio.async(connection_service_worker(self), loop=loop),
            asyncio.async(manage_worker(self), loop=loop),
        ]
        loop.run_until_complete(asyncio.wait(workers))


# TODO: exchange by multiprocessing.Queue
class ServerWorkerProxy(object):
    """
    In worker server manage access object.
    """

    def get_logger(self):
        pass

    def get_timer(self):
        pass

    def get_progress_bar(self, task_id):
        pass
