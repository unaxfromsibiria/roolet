# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import json


def method_path(data):
    return str(data or '')


def str_data(data):
    if isinstance(data, dict):
        data = json.dumps(data).replace('"', '\\"')

    return str(data or '')
