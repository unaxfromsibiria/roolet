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

// get simple command
func NewCommand(cid, method, data string) *Command {
	cmd := Command{Method: method, Params: MethodParams{Data: data, Cid: cid}}
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
