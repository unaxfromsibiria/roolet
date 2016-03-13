# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import json

from .helpers import method_path, str_data
from .protocol import CommandTargetType


class MetaOnceObject(type):
    """
    Once system object.
    """

    _classes = dict()

    def __call__(self, *args, **kwargs):
        cls = str(self)
        if cls not in self._classes:
            this = super().__call__(*args, **kwargs)
            self._classes[cls] = this
        else:
            this = self._classes[cls]
        return this


class Command(object):
    target = None
    cid = None
    data = None
    method = None

    _types = {
        'target': CommandTargetType,
        'cid': str_data,
        'data': str_data,
        'method': method_path,
    }

    @classmethod
    def create(cls, cmd=None, **data):
        if not isinstance(cmd, cls):
            cmd = cls()

        for attr, value in data.items():
            if '__' in attr:
                continue

            if hasattr(cls, attr):
                if attr in cls._types:
                    setattr(cmd, attr, cls._types[attr](value))
                else:
                    setattr(cmd, attr, value)
        return cmd

    def __init__(self, **data):
        self.__class__.create(cmd=self, **data)

    def __str__(self, *args, **kwargs):
        return self.as_json().replace(
            '{', '{\n ').replace(',', ',\n').replace('}', '\n}')

    def as_json(self):
        # not use json
        return (
            '{{"target": {}, "cid": "{}", "data": "{}", '
            '"method": "{}"}}').format(
                self.target.value if self.target else 0,
                self.cid or '',
                self.data or '',
                self.method or '')


class CommandBuilder(object):

    _data = None
    _command_data = None
    cls_command = Command

    def __init__(self):
        self._data = ''

    def append(self, data):
        try:
            obj = json.loads(data)
        except ValueError:
            self._data += data
            obj = None
        else:
            assert self._command_data is None
            self._command_data = obj

        if not obj:
            try:
                obj = json.loads(self._data)
            except ValueError:
                # to next
                pass
            else:
                assert self._command_data is None
                self._command_data = obj

    def is_done(self):
        return isinstance(self._command_data, dict)

    def get_command(self):
        if self._command_data:
            if not isinstance(self._command_data, dict):
                raise ValueError(
                    'Format error, incoming object is {}'.format(
                        self._command_data.__class__))

            try:
                return self.cls_command(**self._command_data)
            finally:
                self._command_data = None
