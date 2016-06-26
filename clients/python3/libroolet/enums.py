# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

from enum import Enum


class AnswerErrorCode(Enum):
    # server
    InternalProblem = 1
    CommandFormatWrong = 2
    MethodParamsFormatWrong = 3
    MethodAuthFailed = 4
    AccessDenied = 5
    UnexpectedValue = 6
    RemouteMethodNotExists = 7
    AllServerBusy = 8
    # client only
    IncorrectFormat = 100
    ResultTimeout = 101
    NoMethod = 102
    ExecError = 103
    FormatError = 104


class GroupConnectionEnum(Enum):
    Server = 1
    Client = 2
    WsClient = 3


class ProcCmdType(Enum):
    Exit = 0
    Complete = 1
    Wait = 3
    Exec = 4
    Result = 5
    Progress = 6
