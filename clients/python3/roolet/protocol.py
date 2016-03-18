# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import logging
import string
from enum import Enum
from hashlib import sha256, md5
from random import SystemRandom

_cr_methods = {
    'sha256': sha256,
    'md5': md5,
}


class ServiceGroup(Enum):
    service = 1
    server = 2
    web = 3


class CommandTargetType(Enum):
    exit = 0
    auth_request = 1
    auth = 2
    client_data = 3
    wait_command = 4
    server_status = 5
    methods_registration = 6
    call_method = 7
    wait_free = 8
    problem = 9
    ok = 10
    server_call = 11
    ping = 12
    get_result = 13


class Protocol(object):

    _handlers = {}
    _options = {}
    _logger = None

    def __init__(self, **options):
        self._options.update(**options)

    @classmethod
    def add_handler(cls, target, handler):
        assert callable(handler)
        cls._handlers[target] = handler

    def processing(self, command, manager):
        if not self._logger:
            self._logger = logging.getLogger(
                self._options.get('logger_name'))

        handler = self._handlers.get(command.target)

        if not callable(handler):
            raise NotImplementedError(
                'Unknown target {}!'.format(command.target))

        return handler(command, manager, self._options, self._logger)


# # handlers # #
def auth_request(command, manager, options, logger):
    key = command.data
    variants = string.digits + string.ascii_letters
    rand = SystemRandom()
    size = len(key)
    client_solt = ''.join(rand.choice(variants) for _ in range(size))
    content = '{}{}{}'.format(options.get('secret'), client_solt, key)
    _hash = _cr_methods.get(options.get('hash_method'))
    if hash:
        content = _hash(bytes(content, 'utf-8')).hexdigest()
    else:
        content = 'no method'

    return command.create(
        target=CommandTargetType.auth,
        data='{}:{}'.format(content, client_solt))


def send_client_data(command, manager, options, logger):
    manager.setup_cid(command.cid)
    return command.create(
        target=CommandTargetType.client_data,
        data={
            'workers': options.get('workers') or 1,
            'group': ServiceGroup.server.value,
        })


def send_api_methods(command, manager, options, logger):
    return command.create(
        target=CommandTargetType.methods_registration,
        data={
            'methods': manager.get_public_methods(),
        })


def start_info(command, manager, options, logger):
    return None


def send_status(command, manager, options, logger):
    return command.create(
        target=CommandTargetType.server_status,
        data={
            'status': manager.get_status().value,
        })

# # link # #
Protocol.add_handler(CommandTargetType.auth_request, auth_request)
Protocol.add_handler(CommandTargetType.client_data, send_client_data)
Protocol.add_handler(CommandTargetType.methods_registration, send_api_methods)
Protocol.add_handler(CommandTargetType.wait_command, start_info)
Protocol.add_handler(CommandTargetType.server_status, send_status)
