# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria


class ConfigError(ValueError):

    def __init__(self, msg):
        super(ConfigError, self).__init__(
            'Open config problem: {}'.format(msg))


class ExecuteError(Exception):
    pass
