#!/usr/bin/env python3
import json
import io
import re
import socket
import string
import time
from _socket import SHUT_WR
from copy import copy


def make_jsonrpc_command(method, params):
    cmd = {
        'id': 0,
        'jsonrpc': '2.0',
        'method': method or '',
        'params': copy(params) or {}
    }
    json_data = params.get('json')
    if json_data and not isinstance(json_data, str):
        cmd['params']['json'] = json.dumps(json_data)

    content = json.dumps(cmd)
    print('-> {}'.format(content))
    content = '{}\n'.format(content)
    return content.encode('utf8')

token = (
    'dGVzdA==.dGVzdA==.a01oVXBHYzB4U1VPM0VvNUxSNFJibXJWV05yW'
    'kkxQU5VS3BXWDVTcXl2N01hVFFySDhNYXU5ZmU4ZlE5R2VnMlI3SmZn'
    'VHo4cU5hLUk5ajVkbS1HY0kxUVdEMWpDSDVVVGhXZkpwZ3hyS1FoaHp'
    'BQ0Y4S01tVy1HajlfNy1WeUVMZVpxQWE2bHc5a2JTZWhtbmlKVzR5d0'
    'tMejRJX0pNeDh1bXZSN3NzNXZn')


def send_ping(result, state):
    time.sleep(2)
    return ('ping', {})


def send_client_registration(result, state):
    return (
        'registration', {
            'json': {
                'group': 1,
                'methods': ['calc_sum', 'calc_composition', 'calc_power'],
            }
    })

def send_up_status(result, state):
    return ('statusupdate', {'data': '1'})

methods = {
    'auth': send_client_registration,
    'registration': send_up_status,
    'statusupdate': send_ping,
    'ping': send_ping,
}


def get_new_command(method, result, state):
    answer = None
    proc = methods.get(method)
    if callable(proc):
        answer = proc(result, state)
    else:
        print('Unknown method: {}'.format(method))
        state.update(exit=True)
    return answer


def run():
    current_command = None
    conn = socket.socket()
    conn.connect(('127.0.0.1', 7551))
    conn.send(make_jsonrpc_command(
        'auth', {'json': {'key': 'test1.key'}, 'data': token}))

    tmp = conn.recv(1024)
    state = dict(exit=False)
    last_mehod = 'auth'

    while tmp:
        answer = None
        if tmp:
            print('-----------------------')
            try:
                iter_lines = tmp.decode("utf-8")
            except Exception as err:
                print(err)
                print(tmp)

            if iter_lines:
                for iter_data in iter_lines.split('\n'):
                    if not iter_data:
                        continue

                    try:
                        data = json.loads(iter_data)
                    except Exception as err:
                        data = {}
                        print('Error: ', err)
                        print('>>{}<<'.format(iter_data))
                        break
                    else:
                        print(data)
                        result = data.get('result')
                        error = data.get('error')
                        if result and not error:
                            try:
                                result = json.loads(result)
                            except Exception as err:
                                print(err)
                                result = {}
                            answer = get_new_command(last_mehod, result, state)
                            if answer:
                                last_mehod, __ = answer
                        else:
                            result = {}
                            if error:
                                print('Terminated with error:')
                                print(error)
                                state.update(exit=True)

        if state.get('exit'):
            break

        if answer:
            answer_content = make_jsonrpc_command(*answer)
            conn.send(answer_content)

        tmp = conn.recv(1024)


    conn.close()


run()
