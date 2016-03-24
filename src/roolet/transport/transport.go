package transport


import (
	"encoding/json"
	"fmt"
)

const (
	JSONRpcVersion = "2.0"
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
	Cid string`json:"cid"`
	// base64
	Data string `json:"data"`
	// json
	Json string `json:"json"`
}

type Command struct {
	Jsonrpc string `json:"jsonrpc"`
	Id int `json:"id"`
	// used
	Method string `json:"method"`
	Params MethodParams
}

type ErrorDescription struct {
	Code int `json:"code"`
	Message string `json:"message"`
}

type Answer struct {
	Jsonrpc string `json:"jsonrpc"`
	Id int `json:"id"`
	Result string `json:"result"`
	Error ErrorDescription
}

// get simple command
func NewCommand(id int, cid, method, data string) *Command {
	cmd := Command{
		Id: id,
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

func (cmd *Command) Load(data []byte) error {
	return json.Unmarshal(data, cmd)
}

func (cmd Command) String() string {
	return fmt.Sprintf("Commad(id=%d, cid=%s, method=%s)", cmd.Id, cmd.Params.Cid, cmd.Method)
}

func NewErrorAnswer(id, code int, msg string) *Answer {
	result := Answer{
		Id: id,
		Jsonrpc: JSONRpcVersion,
		Error: ErrorDescription{Code: code, Message: msg}}
	return &result
}

func NewAnsew(id int, res string) *Answer {
	result := Answer{Id: id, Result: res, Jsonrpc: JSONRpcVersion}
	return &result
}
