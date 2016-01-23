package options

import (
    "fmt"
    "io/ioutil"
    "roolet/rllogger"
    "encoding/json"
)

type SysOption struct {
    Port int
    Addr string
    WsPort int
    WsAddr string
    BufferSize int
    Node string
	Workers int
	Statistic bool
	StatisticCheckTime int
	CountWorkerTime bool
	KeySize int
	Secret string
}

func (option SysOption) Socket() string {
    return fmt.Sprintf("%s:%d", option.Addr, option.Port)
}

func (option SysOption) String() string {
    return fmt.Sprintf(
        "\tservice=%s:%d\n\twebsocket=%s:%d\n\tbuffersize=%d\n\tnode=%s\n\tworkers=%d\n\tstatistic=%t\n\tchecktime=%d\n",
        option.Addr, option.Port, option.WsAddr, option.WsPort, option.BufferSize,
        option.Node, option.Workers, option.Statistic, option.StatisticCheckTime)
}

func (option SysOption) GetDefaultKeySize() int {
    return option.KeySize
}

func (option SysOption) GetSecretKey() string {
    return option.Secret
}

func (option SysOption) GetCidConstructorData() (int, string) {
    return option.KeySize, option.Node
}

type OptionLoder interface {
    Load(useLog bool) (*SysOption, error)
}

type JsonOptionSrc struct {
    FilePath string
}

func (src JsonOptionSrc) Load(useLog bool) (*SysOption, error) {
    content, err := ioutil.ReadFile(src.FilePath)
    if err != nil {
        if useLog {
            rllogger.Outputf(rllogger.LogWarn, "Load: %s", err)
        }
        return nil, err
    }
    option := SysOption{}
    err = json.Unmarshal(content, &option)
    if err != nil{
        if useLog {
            rllogger.Outputf(rllogger.LogWarn, "Load: %s", err)
        }
        return nil, err
    }
    return &option, nil
}
