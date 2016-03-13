# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import logging
import json
import os
import sys
import traceback

from .common import MetaOnceObject
from .exceptions import ConfigError

ENV_PATH_VAR = 'ROOLET_CONF'


def _exception_to_err(err):
    if isinstance(err, Exception):
        err = '{}: {}\n{}'.format(
            err.__class__.__name__,
            err,
            traceback.format_exc())

    return err


class LoggerWrapper:
    _logger = None
    _prefix = None

    def __init__(self, prefix):
        self._prefix = prefix

    def setup(self, logger):
        self._logger = logger

    def _add_prefix(self, msg):
        return '<{}>: {}'.format(self._prefix, _exception_to_err(msg))

    def info(self, msg):
        self._logger.info(self. _add_prefix(msg))

    def debug(self, msg):
        self._logger.debug(self. _add_prefix(msg))

    def warn(self, msg):
        self._logger.warn(self. _add_prefix(msg))

    def error(self, msg):
        self._logger.error(self. _add_prefix(msg))

    def critical(self, msg):
        self._logger.critical(self. _add_prefix(msg))


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


class Configuration(dict):

    __metaclass__ = MetaOnceObject
    _logger = None
    _logger_wrapper_cls = None

    default = {
        'workers': 1,
        'addr': '127.0.0.1',
        'port': '7551',
        'iter': 0.2,
        'status_time': 2,
        'log': '/var/log/roolet.log',
        'logger_name': 'roolet',
        'log_level': 'DEBUG',
    }

    def __init__(self, auto=True, env_var=ENV_PATH_VAR, logger=None, **kwargs):
        super(Configuration, self).__init__(self.default)
        if auto:
            assert isinstance(env_var, str) and env_var
            path = os.environ[env_var]
            try:
                with open(path) as conf:
                    options = json.loads(conf.read())
            except Exception as err:
                raise ConfigError(err)
            else:
                assert isinstance(options, dict), 'Dict in json?'
                self.update(**options)
        self.update(**kwargs)
        if logger:
            self._logger = logger
        else:
            self._logger = None

    def get_logger(self, format_method=None, wraper=None):
        if self._logger:
            logger = self._logger
        else:
            level = getattr(logging, self.get('log_level'), logging.ERROR)
            logger = logging.getLogger(self.get('logger_name'))
            logger.setLevel(level)
            handler = logging.StreamHandler(sys.stdout)
            handler.setLevel(level)
            file_handler = logging.FileHandler(self.get('log'))
            file_handler.setLevel(level)
            if callable(format_method):
                format_method(handler, file_handler)
            else:
                _default_log_formater(handler, file_handler)
            logger.addHandler(handler)
            logger.addHandler(file_handler)
            self._logger = logger

        if isinstance(wraper, LoggerWrapper):
            wraper.setup(logger)
            logger = wraper

        return logger

    @property
    def iter_time(self):
        return self.get('iter')
