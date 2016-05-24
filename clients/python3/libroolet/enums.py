# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

from enum import Enum


class AnswerErrorCode(Enum):
    InternalProblem = 1
    CommandFormatWrong = 2
    MethodParamsFormatWrong = 3
    MethodAuthFailed = 4
    AccessDenied = 5
    UnexpectedValue = 6
    RemouteMethodNotExists = 7
    AllServerBusy = 8


class GroupConnectionEnum(Enum):
    Server = 1
    Client = 2
    WsClient = 3
