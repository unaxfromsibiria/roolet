# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import base64
import json
import pickle

from .enums import AnswerErrorCode

JSON_RPC_VERSION = '2.0'
# TODO: replace from options
encoding = "utf-8"


def _b64_convert(content):
    return (content or b'').decode(encoding)


class DataFormatError(Exception):

    def __init__(self, base_err):
        if isinstance(base_err, Exception):
            msg = '{}: {}'.format(base_err.__class__.__name__, base_err)
        else:
            msg = 'Format problem: {}'.format(base_err)

        super(DataFormatError, self).__init__(msg)


class ProcessingLogicError(Exception):
    pass


class BaseTransportUnit(object):

    fields = None
    # fields = (
    #    ('class field name', 'JSON field name', convert_method, default),
    #    ('class field name', 'level0.level1', convert_method, default),
    # )

    def __init__(self, **data):
        for key, value in data.items():
            setattr(self, key, value)

    def _set_base64_data(self, field, obj):
        if isinstance(obj, bytes):
            setattr(self, field, obj)
        elif obj:
            try:
                data = pickle.dumps(obj)
                data = base64.encodebytes(data)
            except Exception as err:
                raise DataFormatError(err)
            else:
                setattr(self, field, data)

    def _get_base64_data(self, field):
        value = getattr(self, field, None)
        if value and isinstance(value, bytes):
            try:
                data = base64.decodebytes(value)
                data = pickle.loads(data)
            except Exception as err:
                raise DataFormatError(err)
            else:
                return data

    def _set_json_data(self, field, obj):
        if isinstance(obj, dict):
            try:
                setattr(self, field, json.dumps(obj))
            except Exception as err:
                raise DataFormatError(err)
        elif isinstance(obj, str):
            setattr(self, field, obj)
        else:
            raise DataFormatError('Value type must be a dict.')

    def _get_json_data(self, field):
        data = getattr(self, field, None)
        if data and isinstance(data, str):
            try:
                data = json.loads(data)
            except Exception as err:
                raise DataFormatError(err)
            else:
                return data

    def setup(self, json_obj):
        assert isinstance(json_obj, dict)
        for key, value in json_obj.items():
            setattr(self, key, value)

    def as_json(self):
        result = {'jsonrpc': JSON_RPC_VERSION}
        for attr, field, convert, default in self.fields:
            value = (
                convert(getattr(self, attr))
                if callable(convert) else
                getattr(self, attr)) or default

            if '.' in field:
                deep_fields = field.split('.')
                level_result = result
                for level, deep_field in enumerate(deep_fields):
                    if level == len(deep_fields) - 1:
                        level_result[deep_field] = value
                    else:
                        if deep_field not in level_result:
                            level_result[deep_field] = {}

                        level_result = level_result[deep_field]
            else:
                result[field] = value

        return json.dumps(result)


class Command(BaseTransportUnit):

    fields = (
        ('id', 'id', None, 0),
        ('method', 'method', None, ''),
        ('task', 'params.task', None, ''),
        ('cid', 'params.cid', None, ''),
        ('_data', 'params.data', _b64_convert, ''),
        ('_json', 'params.json', None, '')
    )

    id = None
    method = None
    task = None
    cid = None
    _data = None
    _json = None

    def setup(self, json_obj):
        assert isinstance(json_obj, dict)
        self.id = json_obj.get('id')
        self.method = json_obj.get('method')
        params = json_obj.get('params')
        assert isinstance(params, dict)
        self.task = params.get('task')
        self.cid = params.get('cid')
        self._data = (params.get('data') or '').encode(encoding)
        self._json = params.get('json')

    @property
    def data(self):
        return self._get_base64_data('_data')

    @data.setter
    def data(self, obj):
        self._set_base64_data('_data', obj)

    @property
    def json(self):
        return self._get_json_data('_json')

    @json.setter
    def json(self, obj):
        self._set_json_data('_json', obj)


class Answer(BaseTransportUnit):
    fields = (
        ('id', 'id', None, 0),
        ('result', 'result', _b64_convert, ''),
        (
            '_error_code',
            'error.code',
            lambda err: err.value if isinstance(err, AnswerErrorCode) else err,
            0
        ),
        ('_error_msg', 'error.message', None, ''),
    )

    id = None
    _result = None
    _error_code = 0
    _error_msg = None

    def setup(self, json_obj):
        assert isinstance(json_obj, dict)
        self.id = json_obj.get('id')
        self.result = json_obj.get('result')
        error = json_obj.get('error')
        if error:
            assert isinstance(error, dict)
            self._error_code = error.get('code')
            self._error_msg = error.get('message')

    @property
    def result(self):
        return self._get_base64_data('_result')

    @result.setter
    def result(self, obj):
        self._set_base64_data('_result', obj)

    @property
    def error(self):
        return {
            'code': self._error_code or 0,
            'message': self._error_msg or '',
        }

    @error.setter
    def error(self, value):
        if isinstance(value, (list, tuple)):
            self._error_code, self._error_msg = value
        elif isinstance(value, dict):
            self._error_code = value.get('code')
            self._error_msg = value.get('message')
        else:
            raise DataFormatError('Error data format error.')

    def result_as_json(self):
        try:
            return json.loads(self.result or 'null')
        except ValueError:
            pass


class UnitBuilder(object):

    _data = None
    _unit_cls = None
    _result = None

    def __init__(self, unit_cls):
        self._data = ''
        assert issubclass(unit_cls, BaseTransportUnit)
        self._unit_cls = unit_cls

    def append(self, json_content_part):
        try:
            obj = json.loads(json_content_part)
        except ValueError:
            self._data += json_content_part
            obj = None
        else:
            if self._result:
                raise ProcessingLogicError(
                    'Processing result still exists.')
            self._result = obj

        if not obj:
            try:
                obj = json.loads(self._data)
            except ValueError:
                pass
            else:
                if self._result:
                    raise ProcessingLogicError(
                        'Processing result still exists.')

                self._result = obj

        if self._result:
            self._data = ''

    def is_done(self):
        return bool(self._result and isinstance(self._result, dict))

    def _fill(self, unit):
        try:
            unit.setup(self._result)
        except Exception as err:
            raise DataFormatError(err)

    def get_unit(self):
        unit = self._unit_cls()
        if self._result:
            self._fill(unit)
            self._result = None
        return unit
