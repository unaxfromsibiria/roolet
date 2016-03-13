# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

from enum import Enum

from .common import Command, MetaOnceObject
from .protocol import CommandTargetType, Protocol


class WorkerStatusEnum(Enum):
    free = 1
    busy = 2


class Worker(object):
    _status = None
    _index = -1

    def __init__(self, index):
        self._status = WorkerStatusEnum.free
        self._index = index

    @property
    def if_free(self):
        return self._status == WorkerStatusEnum.free


class WorkerPool(list):
    _state_change_lock = False

    def __init__(self, size):
        super().__init__(
            Worker(index)
            for index in range(1, size + 1))

    def has_free(self):
        return (
            self._state_change_lock and
            any(item.is_free for item in self))


class CommandExecuter(object):

    __metaclass__ = MetaOnceObject

    encoding = None
    cls_command = Command
    cls_protocol = Protocol
    _protocol = None
    _manager = None
    _methods = {}
    _workers = None

    def setup(self, configuration, manager, encoding='utf-8'):
        self.encoding = encoding
        self._manager = manager
        self._protocol = self.cls_protocol(**configuration)
        # external service
        manager.service_methods['public_methods'] = lambda: list(self._methods.keys())
        self._workers = workers = WorkerPool(configuration.get('workers') or 1)

        manager.service_methods['has_free_workers'] = lambda: workers.has_free()

    @classmethod
    def get_hello(cls):
        return cls.cls_command(
            target=CommandTargetType.auth_request).as_json()

    @classmethod
    def get_error(cls, err):
        return cls.cls_command(
            data='{}: {}'.format(err.__class__.__name__, err),
            target=CommandTargetType.exit).as_json()

    def run(self, command, as_data=True):
        answer = self._protocol.processing(command, self._manager)
        if answer:
            cid = self._manager.get_cid()
            if cid:
                answer.cid = cid
            if as_data:
                answer = answer.as_json()

        return answer

    def append_public_method(self, name, method):
        if callable(method) and name:
            assert name not in self._methods
            self._methods[name] = method

    def get_status_command(self):
        return self.cls_command(
            target=CommandTargetType.server_status).as_json()


def registration_method(method, name=None):
    """
    api for registration method
    """
    executer = CommandExecuter()
    if not name:
        if method.__module__ == '__main__':
            name = method.__name__
        else:
            name = '{}.{}'.format(
                method.__module__, method.__name__)
    executer.append_public_method(name, method)
