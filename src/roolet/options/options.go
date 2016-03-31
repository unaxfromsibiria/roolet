package options

import (
	"crypto/rsa"
    "fmt"
    "io/ioutil"
    "roolet/rllogger"
    "encoding/json"
    "errors"
    "strings"
    "github.com/dgrijalva/jwt-go"
)

const (
	pubKeyFileName = "key.pub"
	privKeyFileName = "key.priv"
)

type SysOption struct {
    Port int `json:"port"`
    Addr string `json:"addr"`
    WsPort int `json:"ws_port"`
    WsAddr string `json:"ws_addr"`
    BufferSize int `json:"buffer_size"`
    Node string `json:"node"`
	Workers int `json:"workers"`
	Statistic bool `json:"statistic"`
	StatisticFile string `json:"statistic_file"`
	StatisticCheckTime int `json:"statistic_check_time"`
	CountWorkerTime bool `json:"count_worker_time"`
	KeySize int `json:"key_size"`
	Secret string `json:"secret"`
	StatusCheckPeriod int `json:"status_check_period"`
	Passphrase string `json:"passphrase"`
	KeyAlgorithm string `json:"key_algorithm"`
	KeyDir string `json:"key_dir"`
}

func (option SysOption) Socket() string {
    return fmt.Sprintf("%s:%d", option.Addr, option.Port)
}

func (option SysOption) String() string {
    return fmt.Sprintf(
        "\tservice=%s:%d\n\tweb-socket=%s:%d\n\tbuffer size=%d\n\tnode=%s\n\tworkers=%d\n\tstatistic=%t\n\tcheck time=%d\n",
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

func (option SysOption) GetPubKey() (*rsa.PublicKey, error) {
    parts := append(strings.Split(option.KeyDir, "/"), pubKeyFileName)
    if key, err := ioutil.ReadFile(strings.Join(parts, "/")); err != nil {
    	return nil, err
    } else {
    	return jwt.ParseRSAPublicKeyFromPEM(key)
    }
}

func (option SysOption) GetPrivKey() (*rsa.PrivateKey, error) {
    parts := append(strings.Split(option.KeyDir, "/"), privKeyFileName)
    if key, err := ioutil.ReadFile(strings.Join(parts, "/")); err != nil {
    	return nil, err
    } else {
    	return jwt.ParseRSAPrivateKeyFromPEM(key)
    }
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
    if option.Statistic && option.StatisticCheckTime > 0 {
        return &option, nil        
    } else {
        return nil, errors.New("Wrong statistic options.")
    }
}
