# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import asyncio
import base64
import socket
import time

from contextlib import contextmanager
from jwt.algorithms import get_default_algorithms
from random import SystemRandom

from .common import Configuration
from .enums import GroupConnectionEnum, AnswerErrorCode
from .transport import Answer, Command, UnitBuilder, encoding

_random_part_size = 64


class ServerError(Exception):

    def __init__(self, code, message):
        super(ServerError, self).__init__(
            'Server error "{}": {}'.format(code, message))


class ProtocolError(Exception):
    pass


class Connection(object):

    socket_cls = socket.socket

    _conf = None
    _group = None
    _logger = None
    _conn = None
    _sleep = None
    _min_buf_size = 1024
    _auth = False
    _cid = None
    _answer_builder = UnitBuilder(Answer)

    __registration_data = {}

    @classmethod
    @contextmanager
    def open(
             cls,
             group=GroupConnectionEnum.Client,
             conf=None,
             close_after=True,
             sleep_method=time.sleep):
        """
        """
        connection = None
        try:
            if conf is None:
                conf = Configuration()

            connection = cls(conf, group)
            connection._sleep = sleep_method
            connection.up()
            yield connection
        finally:
            if connection and close_after:
                connection.close()

    def __init__(self, config, group):
        assert isinstance(config, Configuration)
        assert isinstance(group, GroupConnectionEnum)
        self._conf = config.as_dict()
        self._logger = config.get_logger()
        self._group = group

    def add_registration_data(self, group, data):
        assert isinstance(group, GroupConnectionEnum)
        assert isinstance(data, dict)
        assert not('group' in data)
        self.__registration_data[group] = data

    def close(self):
        if self._conn:
            self._conn.close()

    def request(self, cmd, do_raise=True):
        assert not self._answer_builder.is_done()
        try:
            if self._cid:
                cmd.cid = self._cid

            self._conn.send(cmd.as_data().encode(encoding=encoding))
            while not self._answer_builder.is_done():
                answer_data = self._conn.recv(self._min_buf_size)
                if answer_data:
                    self._answer_builder.append(answer_data.decode(encoding))
        except (ConnectionRefusedError, ConnectionAbortedError) as err:
            self._logger.error(err)
            self._conn = answer = None
        else:
            answer = self._answer_builder.get_unit()
            if answer and answer.has_error():
                serv_err = ServerError(**err)
                self._logger.fatal(serv_err)
                if do_raise:
                    raise serv_err

        return answer

    def up(self):
        conf = self._conf
        logger = self._logger
        # check and get crypto
        priv_key_path = conf.get('crypto_priv_key_path')
        rand = SystemRandom()
        try:
            with open(priv_key_path, 'r') as key_file:
                priv_rsakey = key_file.read()
                algorithms = get_default_algorithms()
                # 2 random parts
                segments = [
                    base64.urlsafe_b64encode(''.join(
                        # get ascii from [48, 122]
                        chr(rand.randint(48, 123))
                        for _ in range(_random_part_size)
                    ).encode(encoding=encoding))
                    for _ in range(2)
                ]
                algorithm = conf.get('crypto_algorithm')
                try:
                    alg_obj = algorithms[algorithm]
                    signature = alg_obj.sign(
                        b'.'.join(segments),
                        alg_obj.prepare_key(priv_rsakey))

                    segments.append(base64.urlsafe_b64encode(signature))
                    auth_data = b'.'.join(segments)

                except KeyError:
                    raise NotImplementedError(
                        'Algorithm "{}" not supported.'.format(algorithm))

        except (TypeError, FileNotFoundError, PermissionError) as err:
            logger.fatal(
                'Can not use private key "{}", error: {}'.format(
                    priv_key_path, err))
            raise
        else:
            logger.debug('auth data: {}'.format(auth_data))

        logger.info(
            'Connection to {addr}:{port}...'.format(**conf))
        conn = self.socket_cls()
        active = False
        reconnect_delay = self._conf.get('reconnect_delay')

        try:
            self._conn = conn
            while not active:
                try:
                    conn.connect((conf.get('addr'), int(conf.get('port'))))
                except ConnectionRefusedError as err:
                    if reconnect_delay:
                        logger.error(
                            '{} reconnect try after {} sec.'.format(
                                err, reconnect_delay))
                        self._sleep(reconnect_delay)
                    else:
                        raise
                else:
                    active = True
                    # auth
                    cmd = Command(
                        _data=auth_data,
                        method='auth',
                        json={'key': conf.get('crypto_pub_key_name')})

                    answer = self.request(cmd)
                    answer_data = answer.result_as_json()
                    if isinstance(answer_data, dict) and 'auth' in answer_data:
                        self._auth = bool(answer_data.get('auth'))
                        if self._auth:
                            logger.info('Authorization completed.')
                        else:
                            logger.error(
                                'Authorization answer incorrect: {}'.format(
                                    answer._result))
                            raise ServerError(
                                AnswerErrorCode.IncorrectFormat.value,
                                'Authorization answer data incorrect.')
                    else:
                        raise ProtocolError(
                            'Auth answer data has unknown format: {}.'.format(
                                answer.result))
                    # send group info
                    client_info = {'group': self._group.value}
                    adv_data = self.__registration_data.get(self._group)
                    if isinstance(adv_data, dict):
                        client_info.update(adv_data)

                    cmd = Command(
                        method='registration',
                        # some old cid
                        cid=self._cid,
                        json=client_info)

                    answer = self.request(cmd)
                    answer_data = answer.result_as_json() or {}
                    if answer_data.get('ok'):
                        self._cid = answer_data.get('cid')
                    else:
                        ProtocolError(
                            'Registration answer format wrong, data'.format(
                                answer.result))

        except:
            self._conn = None
            raise


class RpcClient(object):
    _connection = None
    _iter_wait = 0.2
    _timeout = 60

    def __init__(self, asyncio_mode=False, **options):
        conn_options = {
            'close_after': False,
        }
        if options:
            conn_options['conf'] = Configuration(env_var=None, **options)

        if asyncio_mode:
            conn_options['sleep_method'] = asyncio.sleep

        with Connection.open(**conn_options) as conn:
            self._connection = conn

    def close(self):
        self._connection.close()

    def call(self, method, params, sync=True):
        cmd = Command(data=params, method=method)
        answer = self._connection.request(cmd, do_raise=False)

        if answer.has_error():
            return answer.err
        else:
            result = answer.result

            if isinstance(result, dict):
                task_id = result.get('task')
                if 'data' in result:
                    # it's immediately answer
                    return result.get('data')

                elif sync and task_id:
                    # wait
                    result = None
                    cmd = Command(task=task_id, method='getresult')
                    delay, wait_time = map(
                        float, (self._iter_wait, self._timeout))

                    while not result:
                        answer = self._connection.request(cmd, do_raise=False)
                        if answer.has_error():
                            return answer.error

                        result = answer.result
                        if isinstance(result, dict):
                            if result.get('exists'):
                                return result.get('data')
                            else:
                                return
                        else:
                            self._connection._logger.warning(answer._result)
                            return {
                                'code': AnswerErrorCode.IncorrectFormat.value,
                                'message': (
                                    'Incorrect result data format '
                                    'for task "{}".').format(task_id),
                            }

                        wait_time -= delay
                        if wait_time <= 0:
                            return {
                                'code': AnswerErrorCode.ResultTimeout.value,
                                'message': (
                                    'Result waiting time limit extended.'),
                            }
                        # time for server processing
                        self._connection._sleep(delay)
                else:
                    return task_id
            else:
                return {
                    'code': AnswerErrorCode.IncorrectFormat.value,
                    'message': 'Incorrect task information returned.',
                }
