package transport

import (
	"encoding/json"
	"fmt"
)

const (
	JSONRpcVersion = "2.0"
	// errors
	ErrorCodeInternalProblem         = 1
	ErrorCodeCommandFormatWrong      = 2
	ErrorCodeMethodParamsFormatWrong = 3
	ErrorCodeMethodAuthFailed        = 4
	ErrorCodeAccessDenied            = 5
	ErrorCodeUnexpectedValue         = 6
	ErrorCodeRemouteMethodNotExists  = 7
)

// helper
func dumps(cmd interface{}, endline bool) (string, error) {
	if data, err := json.Marshal(cmd); err != nil {
		return "", err
	} else {
		if endline {
			return fmt.Sprintf("%s\n", data), nil
		} else {
			return string(data), nil
		}
	}
}

// use JSON-RPC

type MethodParams struct {
	Cid string `json:"cid"`
	// base64
	Data string `json:"data"`
	// json
	Json string `json:"json"`
}

func (parmas MethodParams) String() string {
	return fmt.Sprintf("cid: %s data: %s json: %s", parmas.Cid, parmas.Data, parmas.Json)
}

type Command struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      int    `json:"id"`
	// used
	Method string       `json:"method"`
	Params MethodParams `json:"params"`
}

type ErrorDescription struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (errorDes ErrorDescription) String() string {
	return fmt.Sprintf("Error: %d - %s", errorDes.Code, errorDes.Message)
}

type baseAnswer struct {
	Jsonrpc string `json:"jsonrpc"`
	Id      int    `json:"id"`
	Result  string `json:"result"`
}

type Answer struct {
	baseAnswer
	Error ErrorDescription `json:"error"`
}

func (ans *Answer) Dump() (*string, error) {
	(*ans).Jsonrpc = JSONRpcVersion
	var resultErr error
	var result *string
	if ans.Error.Code > 0 {
		ans.Result = ""
		if data, err := dumps(ans, true); err == nil {
			result = &data
		} else {
			resultErr = err
		}
	} else {
		// no error
		clearAns := (*ans).baseAnswer
		if data, err := dumps(&clearAns, true); err == nil {
			result = &data
		} else {
			resultErr = err
		}
	}
	return result, resultErr
}

func (ans *Answer) DataDump() *[]byte {
	(*ans).Jsonrpc = JSONRpcVersion
	var result *[]byte
	if ans.Error.Code > 0 {
		ans.Result = ""
		if data, err := json.Marshal(ans); err == nil {
			result = &data
		} else {
			result = nil
		}
	} else {
		clearAns := (*ans).baseAnswer
		if data, err := json.Marshal(&clearAns); err == nil {
			result = &data
		} else {
			result = nil
		}
	}
	return result
}

// get simple command
func NewCommand(id int, cid, method, data string) *Command {
	cmd := Command{
		Id:     id,
		Method: method,
		Params: MethodParams{Data: data, Cid: cid}}
	return &cmd
}

func (cmd *Command) Dump() (*string, error) {
	(*cmd).Jsonrpc = JSONRpcVersion
	var resultErr error
	var result *string
	if data, err := dumps(cmd, true); err != nil {
		resultErr = err
	} else {
		result = &data
	}
	return result, resultErr
}

func (cmd *Command) DataDump() *[]byte {
	(*cmd).Jsonrpc = JSONRpcVersion
	var result *[]byte
	if data, err := json.Marshal(cmd); err == nil {
		result = &data
	} else {
		result = nil
	}
	return result
}

func (cmd *Command) Load(data *[]byte) error {
	return json.Unmarshal(*data, cmd)
}

func (cmd *Command) CreateAnswer() *Answer {
	result := Answer{}
	result.Id = (*cmd).Id
	result.Jsonrpc = (*cmd).Jsonrpc
	return &result
}

func (cmd Command) String() string {
	return fmt.Sprintf("Commad(id=%d, cid=%s, method=%s)", cmd.Id, cmd.Params.Cid, cmd.Method)
}

func NewErrorAnswer(id, code int, msg string) *Answer {
	result := Answer{Error: ErrorDescription{Code: code, Message: msg}}
	result.Jsonrpc = JSONRpcVersion
	result.Id = id
	return &result
}

func NewAnswer(id int, res string) *Answer {
	result := Answer{}
	result.Id = id
	result.Result = res
	result.Jsonrpc = JSONRpcVersion
	return &result
}

func ParseCommand(content *[]byte) (*Command, error) {
	cmd := Command{}
	err := cmd.Load(content)
	if err == nil {
		return &cmd, nil
	} else {
		return nil, err
	}
}
