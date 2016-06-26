# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import asyncio

from functools import wraps

from .client import Connection, ServerError
from .common import (
    Configuration, MethodRegistry)
from .enums import GroupConnectionEnum
from .transport import Command, Answer, UnitBuilder, encoding


_defult_reconnect_time = 2.5

read_buffer_size = 8 * 1024


class NotReadyError(Exception):
    pass


def rpc_method_wraper(**options):
    """
    Decorator for server RPC methods.
    Example:

    @rpc_method_wraper(logger=True)
    def rpc_method_sum(x, y, **options):
        logger = options.get('logger')
        result = x + y
        logger.debug('sum: {}'.format(result))
        return result

    Progress bar support for a long time processing.
    Useful where used web-socket client, and as well the opportunity
    call native method 'state' from
    simple client in loop (implemented polling).

    @rpc_method_wraper(progress=True, logger=True)
    def rpc_method_processing(params, **options):
        logger = options.get('logger')
        progress_bar = options.get('progress')

        operation_count = params.n
        progress_bar.total(operation_count)

        for _ in range(opertaion_count):
            ...do some thing
            progress_bar.step()

    Process execute time limiting support.

    @rpc_method_wraper(timeout=1800)
    def rpc_method_processing(params, **options):
        timer = options.get('timer')
        timer.set_message('method extend time limit 30 min')

        operation_count = params.n

        for _ in range(opertaion_count):
            ...do some thing
            timer.rasie_if_exceeded()

    """

    def method_decor(real_method):
        registry = MethodRegistry()

        @wraps(real_method)
        def method_wraper(*args, **kwargs):
            # update kwargs
            _, run_options = registry.get(real_method)

            method_kwargs = {
                key: value
                for key, value in kwargs if key not in ('server', 'task_id')
            }

            task_id = kwargs.get('task_id')
            progress_bar = timeout = None

            if task_id:
                # server proxy
                server = kwargs.get('server')

                if run_options.get('logger'):
                    method_kwargs['logger'] = server.get_logger()

                if run_options.get('progress'):
                    progress_bar = server.get_progress_bar(task_id)
                    method_kwargs['progress_bar'] = progress_bar

                timeout = run_options.get('timeout')

                if timeout:
                    timer = server.get_timer(timeout)
                    method_kwargs['timer'] = timer

            try:
                result = real_method(*args, **method_kwargs)
            finally:
                if progress_bar:
                    progress_bar.done()

                if timeout:
                    timer.stop()

            return result

        # registarion metod
        registry.set(real_method, method_wraper, **options)

    return method_decor


# # server connection processing # #
def _up_connect(conf, state, tmp_data):
    has_connect = False
    options = conf.as_dict()
    logger = conf.get_logger()
    reconnect_time = options.get('reconnect_time') or _defult_reconnect_time
    logger.info('{addr}:{port} <=> ..'.format(**options))

    while not has_connect:
        try:
            result = yield from asyncio.open_connection(
                options.get('addr'),
                int(options.get('port')))
        except Exception as err:
            logger.error('connection error: {}'.format(err))
            yield from asyncio.sleep(reconnect_time)
            has_connect = not(state['active'])
        else:
            has_connect = True
            logger.info('connection establishment')
            if isinstance(tmp_data, dict) and 'auth_data' not in tmp_data:
                tmp_data['auth_data'] = Connection.prepare_auth_token(conf)
            return result


def _init_server(server, reader, writer):
    result = False
    logger = server._conf.get_logger()
    cmd = Command(
        method='auth',
        _data=server._tmp.pop('auth_data'),
        json={'key': server._conf.get('crypto_pub_key_name')})

    writer.write(cmd.as_data().encode(encoding=encoding))
    answer_builder = server.answer_builder
    while not answer_builder.is_done():
        data = yield from reader.read(read_buffer_size)
        if data:
            answer_builder.append(data.decode(encoding))

    answer = answer_builder.get_unit()
    if answer and answer.has_error():
        err = answer.error
        serv_err = ServerError(**err)
        logger.fatal(serv_err)
    else:
        answer_data = answer.result_as_json()
        if isinstance(answer_data, dict) and 'auth' in answer_data:
            result = bool(answer_data.get('auth'))
        else:
            logger.error(
                'Auth answer data has unknown format: {}.'.format(
                    answer.result))

    if result:
        result = False
        client_info = {
            'group': GroupConnectionEnum.Server.value,
            'methods': MethodRegistry().methods_list,
        }
        logger.info(
            'Public methods: {}'.format(
                ', '.join(map('"{}"'.format, client_info['methods']))))

        cmd = Command(
            method='registration',
            # if cid still exists
            cid=server.state.get('cid'),
            json=client_info)

        writer.write(cmd.as_data().encode(encoding=encoding))
        while not answer_builder.is_done():
            data = yield from reader.read(read_buffer_size)
            if data:
                answer_builder.append(data.decode(encoding))

        answer = answer_builder.get_unit()
        if answer and answer.has_error():
            err = answer.error
            serv_err = ServerError(**err)
            logger.fatal(serv_err)
        else:
            answer_data = answer.result_as_json() or {}
            if answer_data.get('ok'):
                result = True
                server.state['cid'] = answer_data.get('cid')
            else:
                logger.error(
                    'Registration answer format wrong, data: {}'.format(
                        answer.result))

    return result


@asyncio.coroutine
def _processing_connection(server):
    logger = server._conf.get_logger()
    options = server._conf.as_dict()
    iter_delay = options.get('iter_delay') or 0.1
    logger.info((
        'Start connection server to'
        ' {addr}:{port}').format(**options))

    while server.state['active']:
        try:
            reader, writer = yield from _up_connect(
                conf=server._conf,
                state=server.state,
                tmp_data=server._tmp)
        except TypeError:
            yield from asyncio.sleep(iter_delay)
        else:
            init_done = yield from _init_server(server, reader, writer)

            if init_done:
                logger.info('Authorization completed.')
                # processing loop
                while server.state['active']:
                    # TODO: wait output Queue
                    # TODO: wait command to input Queue
                    # TODO: get exception of broken connection
                    # and goto prev yield for new reader, writer
                    yield from asyncio.sleep(1)

            else:
                server.state['active'] = False
                logger.error(
                    'Authorization answer incorrect.')

            yield from asyncio.sleep(iter_delay)

    logger.info('processing connection out')


class Server:
    state = answer_builder = None
    _conf = None
    _state = None
    _buffer_size = 0
    _reconnect_time = 2.5
    __has_setup = False
    _handlers = {}
    _in_q = _out_q = None

    _tmp = {}

    def __init__(
            self,
            unit_builder_cls=UnitBuilder,
            buffer_size=1024,
            conf=None):
        """
        """

        if MethodRegistry.is_empty():
            raise NotReadyError(
                'No one public method. Server must have methods.')

        self.state = {'active': True}
        if not conf:
            conf = Configuration()

        # crypto data
        self._tmp['auth_data'] = Connection.prepare_auth_token(conf)
        self.__has_setup = False
        self._conf = conf
        self._buffer_size = buffer_size
        self._in_q = asyncio.Queue(buffer_size)
        self._out_q = asyncio.Queue(buffer_size)
        self.answer_builder = unit_builder_cls(Answer)

    def get_connection_queue(self):
        return self._in_q, self._out_q

    def setup(self, coroutines):
        assert not self.__has_setup, 'Once setup expected.'
        self.__has_setup = True

        coroutines.append(
            asyncio.async(_processing_connection(self)))
