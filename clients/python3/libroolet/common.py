# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import logging
import json
import os
import sys

ENV_PATH_VAR = 'ROOLET_CONG'


def _default_log_formater(console_handler, file_handler):
    if console_handler:
        _format = (
            '\x1b[37;40m[%(asctime)s] '
            '\x1b[36;40m%(levelname)8s '
            '%(name)s '
            '\x1b[32;40m%(filename)s'
            '\x1b[0m:'
            '\x1b[32;40m%(lineno)d '
            '\x1b[33;40m%(message)s\x1b[0m')
        formatter = logging.Formatter(
            _format,
            datefmt="%Y-%m-%d %H:%M:%S")
        console_handler.setFormatter(formatter)
    if file_handler:
        _format = (
            '[%(asctime)s] =>'
            '%(levelname)8s '
            '%(name)s '
            '%(filename)s'
            '%(lineno)d '
            '%(message)s')
        formatter = logging.Formatter(
            _format,
            datefmt="%Y-%m-%d %H:%M:%S")
        file_handler.setFormatter(formatter)


class ConfigError(ValueError):

    def __init__(self, msg):
        super(ConfigError, self).__init__(
            'Open configuration problem: {}'.format(msg))


class MetaOnceObject(type):

    _classes = {}

    def __call__(self, *args, **kwargs):
        cls = str(self)
        if cls not in self._classes:
            this = super().__call__(*args, **kwargs)
            self._classes[cls] = this
        else:
            this = self._classes[cls]
        return this


class Configuration(object):

    __metaclass__ = MetaOnceObject

    default_logger_name = 'roolet'

    default = {
        'workers': 1,
        'addr': '127.0.0.1',
        'port': '7551',
        'iter': 0.2,
        'status_time': 2,
        # set logger if used any custom logger
        'logger': None,
        'log': '/var/log/roolet.log',
        'log_level': 'DEBUG',
        # set None for reconnection off
        'reconnect_delay': 1,
        # key crypto algorithm
        'crypto_algorithm': 'RS256',
        # public key file name on server
        'crypto_pub_key_name': 'pub.key',
        # filepath to your client private key
        'crypto_priv_key_path': None,
    }

    _content = None
    _logger = None

    def __init__(self, env_var=ENV_PATH_VAR, **kwargs):
        self._content = dict(self.default)
        if env_var:
            assert isinstance(env_var, str)
            try:
                path = os.environ[env_var]
                with open(path) as conf:
                    options = json.loads(conf.read())
            except KeyError:
                raise ConfigError('Set env variable "{}"'.format(env_var))
            except Exception as err:
                raise ConfigError(err)
            else:
                assert isinstance(options, dict), 'Dict in {}?'.format(path)
                self._content.update(**options)

        else:
            self._content.update(**kwargs)
        # if not used external logger, create local
        if not self._content.get('logger'):
            level = getattr(logging, self.get('log_level'), logging.ERROR)
            logger = logging.getLogger(self.default_logger_name)
            logger.setLevel(level)
            handler = logging.StreamHandler(sys.stdout)
            handler.setLevel(level)
            file_handler = logging.FileHandler(self.get('log'))
            file_handler.setLevel(level)
            _default_log_formater(handler, file_handler)
            logger.addHandler(handler)
            logger.addHandler(file_handler)
            self._logger = logger

    def get(self, key):
        return self._content.get(key)

    def as_dict(self):
        return self._content

    def get_logger(self):
        return self._logger or logging.getLogger(self._content.get('logger'))
