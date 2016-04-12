package coremethods

import (
	"encoding/json"
	"errors"
	"fmt"
	"roolet/connectionsupport"
	"roolet/coreprocessing"
	"roolet/cryptosupport"
	"roolet/options"
	"roolet/rllogger"
	"roolet/transport"
)

type AuthData struct {
	Key   string
	Token string
}

func (auth *AuthData) Load(data string) error {
	if loadErr := json.Unmarshal([]byte(data), auth); loadErr != nil {
		return loadErr
	} else {
		if len(auth.Key) < 1 {
			return errors.New("Empty data for auth procedure.")
		} else {
			return nil
		}
	}
}

func (auth *AuthData) Check(option options.SysOption) error {
	var result error
	if key, err := option.GetClientPubKey(auth.Key); err == nil {
		if err := cryptosupport.Check(key, auth.Token); err != nil {
			result = err
		}
	} else {
		result = err
	}
	return result
}

func newAuthData(params transport.MethodParams) (*AuthData, error) {
	rec := AuthData{}
	err := rec.Load(params.Json)
	if err == nil {
		rec.Token = params.Data
		return &rec, nil
	} else {
		return nil, err
	}
}

type ClientInfo struct {
	Group   int
	Methods []string
}

// return date size in result
func ping(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	var insType int
	var result string
	if cmd, exists := inIns.GetCommand(); exists {
		insType = coreprocessing.TypeInstructionPong
		result = fmt.Sprint(len(cmd.Params.Data) + len(cmd.Params.Json))
	} else {
		insType = coreprocessing.TypeInstructionSkip
	}
	outIns := coreprocessing.NewCoreInstruction(insType)
	if insType == coreprocessing.TypeInstructionPong {
		outIns.SetAnswer(inIns.MakeOkAnswer(result))
	}
	return outIns
}

// check token by client public key
func auth(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	// client sends in params:
	//  - json: {"key": "<key name>"}
	//  - data: "<base64 token string, use JWT protocol (random data inside)>"
	// getting (search on option.KeyDir) client public key by name (in command params)
	handler.Stat.AddOneMsg("auth_request")
	changes := connectionsupport.StateChanges{}
	var result *coreprocessing.CoreInstruction
	var resultErr error
	var errCode int
	if cmd, exists := inIns.GetCommand(); exists {
		if authData, err := newAuthData(cmd.Params); err == nil {
			if err := authData.Check(handler.Option); err != nil {
				errCode = transport.ErrorCodeMethodAuthFailed
				resultErr = err
			}
		} else {
			resultErr = err
			errCode = transport.ErrorCodeMethodParamsFormatWrong
		}
	} else {
		errCode = transport.ErrorCodeCommandFormatWrong
		resultErr = errors.New("Not found command in instruction.")
	}
	var answer *transport.Answer
	var insType int
	if errCode > 0 {
		changes.Auth = false
		answer = inIns.MakeErrAnswer(errCode, fmt.Sprint(resultErr))
		insType = coreprocessing.TypeInstructionProblem
		rllogger.Outputf(rllogger.LogWarn, "Failed auth from %s with error: %s", inIns.Cid, resultErr)
	} else {
		changes.Auth = true
		handler.Stat.AddOneMsg("auth_successfull")
		answer = inIns.MakeOkAnswer("{\"auth\":true}")
		insType = coreprocessing.TypeInstructionPing
		rllogger.Outputf(rllogger.LogDebug, "Successfull auth from %s", inIns.Cid)
	}
	result = coreprocessing.NewCoreInstruction(insType)
	result.SetAnswer(answer)
	result.StateChanges = &changes
	return result
}

func registration(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	insType := coreprocessing.TypeInstructionSkip
	var answer *transport.Answer
	var result *coreprocessing.CoreInstruction
	var errStr string
	errCode := 0
	if cmd, exists := inIns.GetCommand(); exists {
		info := ClientInfo{}
		if loadErr := json.Unmarshal([]byte((*cmd).Params.Json), &info); loadErr == nil {
			if handler.StateCheker.IsAuth(inIns.Cid) {
				switch info.Group {
				case connectionsupport.GroupConnectionClient:
					{
						// TODO
					}
				case connectionsupport.GroupConnectionServer:
					{
						dict := coreprocessing.NewMethodInstructionDict()
						dict.RegisterClientMethods(info.Methods...)
						cidMethods := coreprocessing.NewRpcServerManager()
						cidMethods.Append(inIns.Cid, &(info.Methods))
					}
				case connectionsupport.GroupConnectionWsClient:
					{
						// denied
						errCode = transport.ErrorCodeAccessDenied
						errStr = "Web-socket client don't accepted on simple TCP socket."
					}
				default:
					{
						errCode = transport.ErrorCodeCommandFormatWrong
						errStr = "Unknown group, see the protocol specification."
					}
				}
			} else {
				errCode = transport.ErrorCodeAccessDenied
				errStr = "Access denied."
			}
		} else {
			errCode = transport.ErrorCodeMethodParamsFormatWrong
			errStr = fmt.Sprint(loadErr)
		}
	} else {
		errCode = transport.ErrorCodeCommandFormatWrong
		errStr = "Command is empty."
	}
	if errCode > 0 {
		insType = coreprocessing.TypeInstructionProblem
		answer = inIns.MakeErrAnswer(errCode, errStr)
	}
	result = coreprocessing.NewCoreInstruction(insType)
	result.SetAnswer(answer)
	return result
}

func Setup() {
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionPing, ping)
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionAuth, auth)
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionReg, registration)
}
