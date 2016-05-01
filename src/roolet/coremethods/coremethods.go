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
	"strconv"
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
func ProcPing(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
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
func ProcAuth(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	// client sends in params:
	//  - json: {"key": "<key name>"}
	//  - data: "<base64 token string, use JWT protocol (random data inside)>"
	// getting (search on option.KeyDir) client public key by name (in command params)
	handler.Stat.AddOneMsg("auth_request")
	changes := connectionsupport.StateChanges{
		ChangeType: connectionsupport.StateChangesTypeAuth}
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
		insType = coreprocessing.TypeInstructionOk
		rllogger.Outputf(rllogger.LogDebug, "Successfull auth from %s", inIns.Cid)
	}
	result = coreprocessing.NewCoreInstruction(insType)
	result.SetAnswer(answer)
	result.StateChanges = &changes
	return result
}

func ProcRegistration(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	insType := coreprocessing.TypeInstructionSkip
	var answer *transport.Answer
	var result *coreprocessing.CoreInstruction
	var resultChanges *connectionsupport.StateChanges
	var errStr string
	errCode := 0
	if cmd, exists := inIns.GetCommand(); exists {
		info := ClientInfo{}
		if loadErr := json.Unmarshal([]byte((*cmd).Params.Json), &info); loadErr == nil {
			if (*handler).StateCheker.IsAuth(inIns.Cid) {
				switch info.Group {
				case connectionsupport.GroupConnectionClient:
					{
						changes := connectionsupport.StateChanges{
							ChangeType:            connectionsupport.StateChangesTypeGroup,
							ConnectionClientGroup: connectionsupport.GroupConnectionClient}
						resultChanges = &changes
						answer = inIns.MakeOkAnswer("{\"ok\": true}")
					}
				case connectionsupport.GroupConnectionServer:
					{
						dict := coreprocessing.NewMethodInstructionDict()
						methodsCount := dict.RegisterClientMethods(info.Methods...)
						cidMethods := coreprocessing.NewRpcServerManager()
						cidMethods.Append(inIns.Cid, &(info.Methods))
						answer = inIns.MakeOkAnswer(
							fmt.Sprintf("{\"methods_count\": %d, \"ok\": true}", methodsCount))
						changes := connectionsupport.StateChanges{
							ChangeType:            connectionsupport.StateChangesTypeGroup,
							ConnectionClientGroup: connectionsupport.GroupConnectionServer}
						resultChanges = &changes
					}
				case connectionsupport.GroupConnectionWsClient:
					{
						// denied
						errCode = transport.ErrorCodeAccessDenied
						errStr = "Web-socket client don't accepted on simple TCP socket."
					}
				default:
					{
						errCode = transport.ErrorCodeUnexpectedValue
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
	} else {
		insType = coreprocessing.TypeInstructionOk
	}
	result = coreprocessing.NewCoreInstruction(insType)
	result.SetAnswer(answer)
	result.StateChanges = resultChanges
	return result
}

func ProcUpdateStatus(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	insType := coreprocessing.TypeInstructionSkip
	var answer *transport.Answer
	var result *coreprocessing.CoreInstruction
	var resultChanges *connectionsupport.StateChanges
	var errStr string
	errCode := 0
	if cmd, exists := inIns.GetCommand(); exists {
		// check params.data is number type
		if newStatus, err := strconv.ParseUint(cmd.Params.Data, 10, 16); err == nil {
			changes := connectionsupport.StateChanges{
				ChangeType: connectionsupport.StateChangesTypeStatus,
				Status:     uint16(newStatus)}
			resultChanges = &changes
			answer = inIns.MakeOkAnswer(
				fmt.Sprintf("{\"ok\": true, \"status\": %d}", newStatus))
			insType = coreprocessing.TypeInstructionOk
		} else {
			errCode = transport.ErrorCodeMethodParamsFormatWrong
			errStr = "Status has unexpected type."
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
	result.StateChanges = resultChanges
	return result
}

type RpcAnswerData struct {
	Cid  string `json:"cid"`
	Task string `json:"task"`
}

func (rpcData RpcAnswerData) String() string {
	return fmt.Sprintf("task: %s to: %s", rpcData.Task, rpcData.Cid)
}

// main method for client routing to server methods
func ProcRouteRpc(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	var answer *transport.Answer
	var errStr string
	var answerData string
	insType := coreprocessing.TypeInstructionSkip
	errCode := 0
	// check methods
	if cmd, exists := inIns.GetCommand(); exists {
		cidMethods := coreprocessing.NewRpcServerManager()
		variants := cidMethods.GetCidVariants((*cmd).Method)
		if len(variants) > 0 {
			var freeCid string
			for _, serverCid := range variants {
				if !handler.StateCheker.ClientBusy(serverCid) {
					freeCid = serverCid
					break
				}
			}
			if len(freeCid) > 0 {
				data := RpcAnswerData{
					Cid:  freeCid,
					Task: handler.TaskIdGenerator.CreateTaskId()}
				// TODO: to debug
				rllogger.Outputf(rllogger.LogInfo, "rpc call: '%s()' -> %s", (*cmd).Method, data)
				if strData, err := json.Marshal(data); err == nil {
					answerData = string(strData)
					cidMethods.ResultDirection.SetForTask(data.Task, (*inIns).Cid)
				} else {
					errCode = transport.ErrorCodeInternalProblem
					errStr = fmt.Sprintf("Error dump %T: '%s'", data, err)
				}
			} else {
				errCode = transport.ErrorCodeAllServerBusy
				errStr = fmt.Sprintf("All server busy for method '%s'.", (*cmd).Method)
			}
		} else {
			errCode = transport.ErrorCodeRemouteMethodNotExists
			errStr = fmt.Sprintf("Method '%s' unregistred or workers lost.", (*cmd).Method)
		}
	} else {
		errCode = transport.ErrorCodeCommandFormatWrong
		errStr = "Command is empty."
	}
	if errCode > 0 {
		insType = coreprocessing.TypeInstructionProblem
		answer = inIns.MakeErrAnswer(errCode, errStr)
	} else {
		insType = coreprocessing.TypeInstructionOk
		answer = inIns.MakeOkAnswer(answerData)
	}
	result := coreprocessing.NewCoreInstruction(insType)
	result.SetAnswer(answer)
	return result
}

func ProcCallServerMethod(
	handler *coreprocessing.Handler,
	inIns *coreprocessing.CoreInstruction,
	outIns *coreprocessing.CoreInstruction) []*coreprocessing.CoreInstruction {
	//
	var result []*coreprocessing.CoreInstruction

	if answer, exists := outIns.GetAnswer(); exists {
		if (*answer).Error.Code == 0 {
			if srcCmd, hasCmd := inIns.GetCommand(); hasCmd {
				rpcData := RpcAnswerData{}
				// little overhead - parse JSON
				if loadErr := json.Unmarshal([]byte((*answer).Result), &rpcData); loadErr == nil {
					srcParams := (*srcCmd).Params
					srcParams.Task = rpcData.Task
					// replace cid
					srcParams.Cid = rpcData.Cid
					newCmd := transport.NewCommandWithParams(0, (*srcCmd).Method, srcParams)
					resultIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionExecute)
					resultIns.SetCommand(newCmd)
					result = []*coreprocessing.CoreInstruction{resultIns}
				} else {
					rllogger.Outputf(
						rllogger.LogError,
						"Answer from ProcRouteRpc has incorrect data format, from: %s",
						inIns.Cid)
				}
			} else {
				rllogger.Outputf(rllogger.LogError, "Answer lost in ProcRouteRpc! from: %s", inIns.Cid)
			}
		}
	} else {
		rllogger.Outputf(rllogger.LogError, "Answer lost in ProcRouteRpc! from: %s", inIns.Cid)
	}
	return result
}

func ProcResultReturned(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	var answer *transport.Answer
	var errStr string
	insType := coreprocessing.TypeInstructionSkip
	errCode := 0

	if cmd, exists := inIns.GetCommand(); exists {
		if len((*cmd).Params.Task) < 1 {
			errCode = transport.ErrorCodeMethodParamsFormatWrong
			errStr = "Task Id does not exist."
		}
	} else {
		errCode = transport.ErrorCodeCommandFormatWrong
		errStr = "Command is empty."
	}
	if errCode > 0 {
		insType = coreprocessing.TypeInstructionProblem
		answer = inIns.MakeErrAnswer(errCode, errStr)
	} else {
		insType = coreprocessing.TypeInstructionOk
		answer = inIns.MakeOkAnswer("{\"ok\": true}")
	}
	result := coreprocessing.NewCoreInstruction(insType)
	result.SetAnswer(answer)
	return result
}

func ProcRecordResult(
	handler *coreprocessing.Handler,
	inIns *coreprocessing.CoreInstruction,
	outIns *coreprocessing.CoreInstruction) []*coreprocessing.CoreInstruction {
	//
	var result []*coreprocessing.CoreInstruction
	if answer, _ := outIns.GetAnswer(); (*answer).Error.Code == 0 {
		if srcCmd, hasCmd := inIns.GetCommand(); hasCmd {
			cidMethods := coreprocessing.NewRpcServerManager()
			// task ID must placed in prams.data
			if hasCid, targetCid := cidMethods.ResultDirection.Get((*srcCmd).Params.Data); hasCid {
				// check client group
				if handler.StateCheker.ClientInGroup(targetCid, connectionsupport.GroupConnectionWsClient) {
					// TODO: create answer here and return as result
					// It will be immediately written to web-socket connection 
				} else {
					/// TODO:
					// write result to heap
				}
			} else {
				rllogger.Outputf(rllogger.LogError, "Processing pass for: %s", srcCmd)
			}			
		}
	}
	return result
}

func Setup() {
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionPing, ProcPing, nil)
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionAuth, ProcAuth, nil)
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionReg, ProcRegistration, nil)
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionStatus, ProcUpdateStatus, nil)
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionExternal, ProcRouteRpc, ProcCallServerMethod)
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionResult, ProcResultReturned, ProcRecordResult)
}
