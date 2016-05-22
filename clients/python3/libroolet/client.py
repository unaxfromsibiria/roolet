# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import base64
import socket
import time

from contextlib import contextmanager
from jwt.algorithms import get_default_algorithms
from random import SystemRandom

from .common import Configuration
from .transport import Answer, Command, UnitBuilder, encoding

_random_part_size = 64


class ServerError(Exception):

    def __init__(self, code, message):
        super(ServerError, self).__init__(
            'Server error "{}": {}'.format(code, message))


class ProtocolError(Exception):
    pass


class Connection(object):

    _conf = None
    _logger = None
    _conn = None
    _sleep = None
    _min_buf_size = 1024
    _auth = False

    @classmethod
    @contextmanager
    def open(cls, conf=None, sleep_method=time.sleep):
        connection = None
        try:
            if conf is None:
                conf = Configuration()

            connection = cls(conf)
            connection._sleep = sleep_method
            connection.up()
            yield connection
        finally:
            if connection:
                connection.close()

    def __init__(self, config):
        assert isinstance(config, Configuration)
        self._conf = config.as_dict()
        self._logger = config.get_logger()

    def close(self):
        if self._conn:
            self._conn.close()

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
        conn = socket.socket()
        active = False
        reconnect_delay = self._conf.get('reconnect_delay')

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
                self._conn = conn
                active = True
                # auth
                cmd_json = '{}\n'.format(Command(
                    _data=auth_data,
                    method='auth',
                    json={'key': conf.get('crypto_pub_key_name')}).as_json())

                conn.send(cmd_json.encode(encoding=encoding))
                answer_builder = UnitBuilder(Answer)
                while not answer_builder.is_done():
                    answer_data = conn.recv(self._min_buf_size)
                    if answer_data:
                        answer_builder.append(answer_data.decode(encoding))

                answer = answer_builder.get_unit()
                err = answer.error

                if err and err.get('code'):
                    logger.fatal('Auth filed!')
                    serv_err = ServerError(**err)
                    logger.fatal(serv_err)
                    raise serv_err

                answer = answer.result_as_json()
                if isinstance(answer, dict) and 'auth' in answer:
                    self._auth = bool(answer.get('auth'))
                    if self._auth:
                        logger.info('Authorization completed.')
                    else:
                        logger.error('Authorization failed.')
                        return
                else:
                    raise ProtocolError(
                        'Auth answer data has unknown format: "{}".'.format(
                            answer.result))

                # send group info
                # TODO
