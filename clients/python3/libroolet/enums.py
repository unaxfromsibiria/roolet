# @author: Michael Vorotyntsev
# @email: linkofwise@gmail.com
# @github: unaxfromsibiria

from enum import Enum


class AnswerErrorCode(Enum):
    ErrorCodeInternalProblem = 1
    ErrorCodeCommandFormatWrong = 2
    ErrorCodeMethodParamsFormatWrong = 3
    ErrorCodeMethodAuthFailed = 4
    ErrorCodeAccessDenied = 5
    ErrorCodeUnexpectedValue = 6
    ErrorCodeRemouteMethodNotExists = 7
    ErrorCodeAllServerBusy = 8
