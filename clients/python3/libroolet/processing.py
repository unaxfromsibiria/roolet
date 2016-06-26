# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

import time
import asyncio
import aioprocessing as aioproc
import multiprocessing as mp

from collections import defaultdict

from .common import Configuration, MethodRegistry
from .enums import AnswerErrorCode, ProcCmdType
from .server import Server as ConnectionServer
from .transport import ProcCmd


_min_iter_delay = 0.25


class ProgressProxy:

    update_interval = 0.1

    __send_method = None
    __total = 0
    __step_index = 0
    __step_percent = 0
    __last_time_update = 0

    def __init__(self, send_method):
        assert callable(send_method)
        self.__send_method = send_method

    def _send(self, percent):
        if callable(self.__send_method):
            self.__send_method(int(percent * 100))

    def clear(self):
        self.__total = self.__step_index = 0
        self.__last_time_update = self.__step_percent = 0

    def total(self, value):
        self.clear()
        try:
            self.__total = int(value)
        except (ValueError, TypeError):
            pass
        else:
            self.__step_percent = self.__total / 100.0

    def step(self, delta_step=1):
        self.__step_index += delta_step

        now_time = time.time()
        if now_time - self.__last_time_update >= self.update_interval:
            self.__last_time_update = now_time
            percent = self.__step_index / (self.__step_percent or 1)
            if percent > 99.9:
                percent = 100.0
            self._send(percent)

    def done(self):
        self._send(100)


def _call_method(logger, task, method, params):
    try:
        return {'result': method(**params)}
    except Exception as err:
        logger.error((
            'Execute method "{}" from task {}'
            ' error: {}').format(
                method, task, err))
        return {
            'error': {
                'code': AnswerErrorCode.ExecError.value,
                'message': str(err),
            }
        }


def _worker_handler(index, options, methods, manage_queue, processing_queue):
    conf = Configuration(env_var=None, **options)
    logger = conf.get_logger()
    logger.info('Start worker {}'.format(index))
    active = True
    answer = None
    current_task_data = {'index': index}

    def progress_send(percent_value):
        progress_cmd = {'result': {'value': percent_value}}
        progress_cmd.update(current_task_data)
        manage_queue.put_nowai(ProcCmd(ProcCmdType.Progress, progress_cmd))

    while active:
        answer = None
        try:
            cmd = processing_queue.get_nowait()
        except:
            cmd = None
        else:
            if not isinstance(cmd, ProcCmd):
                logger.error(
                    'Unexpected type cmd: {}'.format(cmd.__class__))
                cmd = None

        if cmd:
            if cmd.has_type(ProcCmdType.Exit):
                active = False
                manage_queue.put_nowait(
                    ProcCmd(ProcCmdType.Complete, {'index': index}))

            elif cmd.has_type(ProcCmdType.Exec):
                method_name = cmd.data.get('method')
                task_id = cmd.data.get('method')
                params = cmd.data.get('params') or {}
                method, default_options = (
                    methods.get(method_name) or (None, None))
                current_task_data.update(task=task_id)
                result = {
                    'index': index,
                    'task': task_id,
                }

                if default_options.get('logger'):
                    params['logger'] = logger

                if default_options.get('progress'):
                    params['progress'] = ProgressProxy(progress_send)

                timeout = default_options.get('timeout')
                if timeout:
                    params['timeout'] = timeout

                if isinstance(params, dict):
                    if callable(method):
                        logger.info(
                            'Run task: {} at worker: {} method: {}'.format(
                                task_id, index, method_name))

                        result.update(_call_method(
                            logger=logger,
                            method=method,
                            params=params,
                            task=task_id))
                    else:
                        result['error'] = {
                            'code': AnswerErrorCode.NoMethod.value,
                            'message': (
                                'Not found method: '
                                '"{}"').format(method_name),
                        }
                else:
                    result['error'] = {
                        'code': AnswerErrorCode.FormatError.value,
                        'message': 'Params format error',
                    }

                answer = ProcCmd(ProcCmdType.Result, result)

        if active:
            if answer is None:
                answer = ProcCmd(ProcCmdType.Wait, {'index': index})

            manage_queue.put_nowait(answer)
            time.sleep(_min_iter_delay)

    logger.info('Stop worker {}'.format(index))


@asyncio.coroutine
def _process_worker(index, pool):

    worker = aioproc.AioProcess(
        target=_worker_handler,
        kwargs={
            'options': pool._conf.as_dict(),
            'index': index,
            'manage_queue': pool._manage_queue,
            'processing_queue': pool._processing_queue,
            'methods': pool._methods,
        })

    worker.start()

    pid = int(worker.pid)
    pool._workers[index] = pid
    pool.logger.info('\tworker {}: {}'.format(index, pid))
    yield from worker.coro_join()


@asyncio.coroutine
def _manage_workers(pool, state, to_connection, from_connection):
    """
    Connection <=> workers
    """
    assert isinstance(pool, WorkerPool)

    workers_pid = pool.get_workers_pid()

    while state['active']:
        try:
            cmd = from_connection.get_nowait()
        except asyncio.QueueEmpty:
            cmd = None

        if cmd:
            yield from pool.send_cmd(cmd)

        cmd = yield from pool.get_answer()

        if cmd:
            if cmd.has_type(ProcCmdType.Complete):
                index = cmd.index
                workers_pid[index] = 0
            elif cmd.has_type(ProcCmdType.Wait):
                # wait
                pool.logger.debug((
                    'process {index} wait command from '
                    'connection').format(**cmd.data))
            else:
                to_connection.put_nowait(cmd)

        state['active'] = any(pid > 0 for __, pid in workers_pid.items())

    pool.logger.info('All workers complited.')


class WorkerPool:
    Q_SIZE = 1024
    logger = None
    _conf = None
    _workers = defaultdict(int)
    _manage_queue = _processing_queue = None
    __done_setup = False
    _methods = None

    def __init__(self, conf, methods):
        assert isinstance(conf, Configuration)
        assert methods and isinstance(methods, dict)
        self._conf = conf
        self._methods = methods
        worker_count = conf.get('workers') or 0
        assert 1 <= worker_count <= 1024
        self.logger = conf.get_logger()

    def setup(self, state, to_connection, from_connection, coroutines):
        assert not self.__done_setup
        self.__done_setup = True
        worker_count = int(self._conf.get('workers'))
        mq = aioproc.AioQueue(self.Q_SIZE)
        pq = aioproc.AioQueue(int(self.Q_SIZE / worker_count))
        self._manage_queue, self._processing_queue = mq, pq

        for idx in range(1, worker_count + 1):
            coroutines.append(
                asyncio.async(_process_worker(idx, self)))

        coroutines.append(
            asyncio.async(
                _manage_workers(
                    self, state, to_connection, from_connection)))

    def get_answer(self):
        return self._manage_queue.coro_get()

    def send_cmd(self, cmd):
        return self._processing_queue.coro_put(cmd)

    def stop(self):
        worker_count = int(self._conf.get('workers'))
        for _ in range(worker_count):
            self.send_cmd(ProcCmd(ProcCmdType.Exit))

    def get_workers_pid(self):
        return dict(self._workers)


def launch(conf=None):
    configuration = Configuration()

    workers = WorkerPool(
        conf=configuration,
        methods=MethodRegistry().as_dict())

    server = ConnectionServer(
        conf=configuration,
        buffer_size=workers.Q_SIZE)

    coroutines = []
    server.setup(coroutines)
    conn_in_q, conn_out_q = server.get_connection_queue()
    workers.setup(server.state, conn_in_q, conn_out_q, coroutines)
    loop = asyncio.get_event_loop()
    loop.run_until_complete(asyncio.wait(coroutines))
    loop.close()
