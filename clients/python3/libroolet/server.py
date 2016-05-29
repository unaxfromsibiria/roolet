# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import asyncio
import weakref

from collections import defaultdict
from copy import copy
from functools import wraps

from .client import Connection as BaseConnection
from .common import (
    Configuration, MetaOnceObject, get_object_path)


class MethodRegistry(object):

    __metaclass__ = MetaOnceObject
    __methods = None
    # {method_path: method}
    __run_options = None
    # method standard options:
    # {
    #     'timeout': None,
    #     'progress': True,
    #     'logger': False,
    # }
    _default_run_options = {
        'timeout': None,
        'progress': True,
        'logger': True,
    }

    def __init__(self):
        self.__methods = {}
        self.__run_options = defaultdict(dict)

    def options_update(self, method, **options):
        if callable(method) or isinstance(method, str) and method:
            if not isinstance(method, str):
                method = get_object_path(method)
            self.__run_options[method].update(**options)

    def setup(self, method, **options):
        self.set(method, method, **options)

    def set(self, target_method, call_method, **options):
        if callable(target_method) and callable(call_method):
            method_path = get_object_path(target_method)
            self.__methods[method_path] = weakref.WeakMethod(call_method)
            if options:
                self.options_update(method_path, **options)
        else:
            raise TypeError('Expected a any callable object.')

    def remove(self, method):
        if callable(method) or isinstance(method, str) and method:
            if not isinstance(method, str):
                method = get_object_path(method)
            if method in self.__methods:
                del self.__methods[method]

    def get(self, method):
        if callable(method) or isinstance(method, str) and method:
            if not isinstance(method, str):
                method = get_object_path(method)

        options = copy(self._default_run_options)
        options.update(self.__run_options.get(method) or {})
        return self.__methods.get(method), options


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


class Connection(BaseConnection):
    pass


class Server(object):
    _connection = None
    # multiprocessing group size
    _worker_pool_size = 2

    def __init__(self, asyncio_mode=False, **options):
        conn_options = {
            'close_after': False,
        }
        if options:
            conf = Configuration(env_var=None, **options)
            worker_pool_size = conf.get('workers')
        else:
            worker_pool_size = options.get('workers')

        self._worker_pool_size = int(
            worker_pool_size or self._worker_pool_size)

        if asyncio_mode:
            conn_options['sleep_method'] = asyncio.sleep

        with Connection.open(**conn_options) as conn:
            self._connection = conn

        self._start_manage_thread()
        self._start_connection_service_thread()
        self._start_workers()

    def _start_manage_thread(self):
        pass

    def _start_connection_service_thread(self):
        pass

    def _start_workers(self):
        pass


# TODO: exchange by multiprocessing.Queue
class ServerWorkerProxy(object):
    """
    In worker server manage access object.
    """

    def get_logger(self):
        pass

    def get_timer(self):
        pass

    def get_progress_bar(self, task_id):
        pass
