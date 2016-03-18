# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import base64
import pickle
import socket
import time
from uuid import uuid4
from random import SystemRandom

from .config import Configuration, LoggerWrapper
from .common import CommandBuilder, Command
from .exceptions import ExecuteError
from .protocol import CommandTargetType, auth_request, ServiceGroup


class Client(object):
    """
    TODO:
    """

    cls_command_builder = CommandBuilder
    command_builder = None
    encoding = None
    _connection = None
    _iter = None
    _timeout = None
    _buffer_size = None
    _cid = None
    _cid_part = None

    def __init__(self, conf=None):
        if not conf:
            conf = Configuration()

        self._conf = conf
        self.encoding = conf.get('encoding') or 'utf-8'
        self._iter = conf.get('iter') or 0.05
        self._timeout = conf.get('timeout') or 60
        if not(1 <= self._timeout <= 60):
            self._timeout = 60

        self._buffer_size = conf.get('buffer_size') or 512
        self._logger = conf.get_logger(
            wraper=LoggerWrapper('client'))
        self.command_builder = self.cls_command_builder()
        rand = SystemRandom()
        self._cid_part = '{:0>4}'.format(hex(rand.randint(0, int('ffff', 16)))[2:])

    def _new_cmd(self, **data):
        return self.command_builder.cls_command(cid=self._cid, **data)

    def _read(self):
        start_time = time.time()
        wait = True

        while wait:
            # socket read timeout is decreases
            socket_timeout = self._timeout - round(time.time() - start_time, 1)
            wait = socket_timeout > 0
            if wait:
                self._connection.settimeout(socket_timeout)
                try:
                    new_data = self._connection.recv(self._buffer_size)
                except Exception as err:
                    self._logger.error(err)
                    wait = False
                else:
                    new_data = new_data.decode(self.encoding)
                    new_data = new_data.split('\n')

                    for line_data in new_data:
                        if not line_data:
                            continue

                        self.command_builder.append(line_data)
                        wait = not self.command_builder.is_done()

                        if not wait:
                            yield self.command_builder.get_command()
                finally:
                    if self._connection:
                        self._connection.settimeout(None)
            if wait:
                time.sleep(self._iter)

    def _send(self, command):
        assert isinstance(command, Command)
        data = bytes('{}\n'.format(command.as_json()), self.encoding)
        self._connection.send(data)

    def _auth(self):
        cmd = self._new_cmd(target=CommandTargetType.auth_request)
        self._send(cmd)
        for cmd in self._read():
            if not cmd:
                continue

            assert cmd.target == CommandTargetType.auth_request
            auth_cmd = auth_request(
                command=cmd, manager=None, options=self._conf, logger=self._logger)
            self._send(auth_cmd)
            for cmd in self._read():
                if self._cid is None:
                    self._cid = cmd.cid
                    self._logger.debug('Received new client ID: {}'.format(cmd.cid))
                return cmd.target == CommandTargetType.client_data

        return False

    def _get_task_id(self):
        return '{}-{}'.format(self._cid_part, uuid4())

    def open(self):
        if self._connection is None:
            self._cid = None
            try:
                self._connection = socket.socket(
                    socket.AF_INET, socket.SOCK_STREAM)
                self._connection.connect((
                    self._conf.get('addr'),
                    int(self._conf.get('port')),
                ))
            except Exception as err:
                self._logger.error(err)
            else:
                if self._auth():
                    # send data
                    cmd = self._new_cmd(
                        data={
                            'group': ServiceGroup.service.value,
                        },
                        target=CommandTargetType.client_data)
                    self._send(cmd)
                    for cmd in self._read():
                        if not cmd:
                            continue
                        if not cmd.target == CommandTargetType.wait_command:
                            self._logger.error(
                                'Protocol changed? Unexpected command: {}'.format(cmd))
                            self._connection.close()
                            self._connection = None
                else:
                    self._logger.error('Auth problem! Check key in configuration.')
                    self._connection.close()
                    self._connection = None

    def is_active(self):
        return bool(self._connection and self._cid)

    def execute(self, method, params=None, progress=True):
        """
        :param str method: remote method name
        :param params: method kwargs
        :param bool progress: use native progress bar support
        :rtype str: return task id
        """

        assert method and isinstance(method, str)
        task_id = self._get_task_id()
        data = {
            'id': task_id,
            'params': None,
            'progress': progress,
        }
        if params:
            data.update(
                params=base64.encodebytes(pickle.dumps(params)))

        cmd = self._new_cmd(
            target=CommandTargetType.call_method, data=data, method=method)
        self._send(cmd)

        for cmd in self._read():
            if cmd.target == CommandTargetType.problem:
                raise ExecuteError(cmd.data)
            elif cmd.target == CommandTargetType.ok:
                return task_id

    def get_result(self, task_id):
        cmd = self._new_cmd(
            target=CommandTargetType.get_result, data=task_id)
        self._send(cmd)

        for cmd in self._read():
            if cmd.target == CommandTargetType.problem:
                raise ExecuteError(cmd.data)
            elif cmd.target == CommandTargetType.ok:
                if cmd.data:
                    # TODO: try except wrapper
                    return pickle.loads(base64.decodebytes(cmd.data))
