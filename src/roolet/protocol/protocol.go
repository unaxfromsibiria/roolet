package protocol

import (
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "roolet/helpers"
    "roolet/rllogger"
    "strings"
)

const (
    // targets
    CmdExit = 0
    CmdAuthRequest = 1
    CmdAuth = 2
    CmdClientData = 3
    CmdWaitCommand = 4
    CmdServerStatus = 5
    CmdPublicMethodsRegistration = 6
    CmdCallMethod = 7
    CmdWaitFree = 8
    CmdProblem = 9
    CmdOk = 10

    // error codes
    ErrCodeUnknown = 0
    ErrCodeAuthFailed = 1
    ErrCodeProtocolProblem = 2
    ErrCodeFormatProblem = 3
    ErrCodeAccessDenied = 4
    ErrCodeImplementationProblem = 5

    // type of client
    GroupClientService = 1
    GroupClientServer = 2
    GroupClientWeb = 3

    // status of client processing
    ClientProcStatusFree = 1
    ClientProcStatusBusy = 2
)

func dumps(cmd interface{}) (string, error) {
    if data, err := json.Marshal(cmd); err != nil {
        return "", err
    } else {
        return fmt.Sprintf("%s\n", data), nil
    }
}

type ContextNewData struct {
    auth bool
    data string
    failed bool
    cid string
    group int
}

func (contextData ContextNewData) Update(context *helpers.ConnectionContext) {
    context.Clear()
    context.SetCid(contextData.cid)
    context.SetupTmpData(contextData.data)
    if contextData.auth {
        context.DoAuth()
    }
    if contextData.group != 0 {
        context.SetClientGroup(contextData.group)
    }
}

type baseCmd struct {
    Cid string
    Target int
    Data string
}

type ClientCmd struct {
    baseCmd
    Method string
}

type ServerCmd struct {
    baseCmd
    contextUpdater helpers.ConnectionContextUpdater
}

func NewServerCmd(target int, cid string) *ServerCmd {
    cmd := ServerCmd{
        baseCmd: baseCmd{Target: target, Cid: cid}}
    return &cmd
}

func NewServerDataCmd(target int, cid string, data *string) *ServerCmd {
    cmd := *(NewServerCmd(target, cid))
    cmd.Data = *data
    return &cmd
}

func NewServerExitCmd() *ServerCmd {
    cmd := ServerCmd{baseCmd: baseCmd{Target: CmdExit}}
    return &cmd
}

func (cmd *ServerCmd) IsEmpty() bool {
    return len((*cmd).Cid) == 0 && (*cmd).Target == 0
}

func (cmd *ServerCmd) TargetIs(target int) bool {
    return (*cmd).Target == target
}

func (cmd *ServerCmd) Dump() (*string, error) {
    var resultErr error
    var result *string
    if data, err := dumps(cmd); err != nil {
        resultErr = err
    } else {
        result = &data
    }
    return result, resultErr
}

func (srcCmd ServerCmd) Copy(newCid string) *ServerCmd {
    cmd := ServerCmd{
        baseCmd: baseCmd{Data: srcCmd.Data, Target: srcCmd.Target, Cid: newCid}}
    return &cmd
}

func (srcCmd *ServerCmd) GetContextUpdater() (bool, helpers.ConnectionContextUpdater) {
    if srcCmd.contextUpdater != nil {
        return true, srcCmd.contextUpdater
    } else {
        return false, nil
    }
}

type InfoState struct {
    Status int
}

func (state InfoState) String() string {
    return fmt.Sprintf("state(status=%d)", state.Status)
}

type CoreMsg struct {
    Group int
    Target int
    Cid string
    State *InfoState
    Method string
    Data []byte
}

func (msg *CoreMsg) setupData(newData string, useBase64 bool) {
    if useBase64 {
        (*msg).Data = make([]byte, base64.StdEncoding.EncodedLen(len(newData)))
        base64.StdEncoding.Encode((*msg).Data, []byte(newData))
    } else {
        (*msg).Data = []byte(newData)
    }
}

func (msg *CoreMsg) GetStatus() int {
    result := ClientProcStatusFree
    if (*msg).State != nil {
        result = (*(*msg).State).Status
    }
    return result
}

func (msg *CoreMsg) IsEmpty() bool {
    return (*msg).Group == 0 && len((*msg).Cid) == 0
}

func (msg CoreMsg) String() string {
    dataSize := len(msg.Data)
    var dataPart string
    if dataSize > 0 {
        dataPart = fmt.Sprintf("%s..", msg.Data[0:8])
    } else {
        dataPart = "empty"
    }
    var statusStr string
    if msg.State != nil {
        statusStr = msg.State.String()
    } else {
        statusStr = "no status"
    }
    return fmt.Sprintf(
        "MSG[%d:%s:%d:%s:%s:%d:%s]",
        msg.Group, msg.Cid, msg.Target, msg.Method, statusStr, dataSize, dataPart)
}

func (msg *CoreMsg) FastAnswer(target int) *ServerCmd {
    cmd := ServerCmd{
        baseCmd: baseCmd{Cid: (*msg).Cid, Target: target}}
    return &cmd
}

func NewCoreMsg(group int, cmd *ClientCmd) (*CoreMsg, error) {
    msg := CoreMsg{
        Target: (*cmd).Target,
        Group: group,
        Cid: (*cmd).Cid,
        Method: (*cmd).Method}
    var err error
    msg.setupData((*cmd).Data, false)
    if msg.Target == CmdServerStatus {
        // set status from data in msg
        state := InfoState{}
        loadErr := json.Unmarshal(msg.Data, &state)
        if loadErr != nil {
            err = loadErr
        } else {
            switch state.Status {
                case ClientProcStatusFree, ClientProcStatusBusy: msg.State = &state
                default: err = errors.New("Unknown status!")
            }
        }
    }
    return &msg, err
}

type clientInfo struct {
    Group int
}

func NewClientInfo(content *[]byte) (*clientInfo, error) {
    info := clientInfo{}
    err := json.Unmarshal(*content, &info)
    return &info, err
}

type HandlerParamsReader interface {
    GetDefaultKeySize() int
    GetSecretKey() string
    GetCidConstructorData() (int, string)
}

type CmdHandler func(
    cmd *ClientCmd,
    context *helpers.ConnectionContext,
    option HandlerParamsReader) (*ServerCmd, error)

func (cmd *baseCmd) RequiredClose() bool {
    return (*cmd).Target == CmdExit
}

func NewClientCmd(content *[]byte) (*ClientCmd, error) {
    cmd := ClientCmd{}
    err := json.Unmarshal(*content, &cmd)
    return &cmd, err
}

func FastErrorAnswer(err interface{}, errCode int) *string {
    result := fmt.Sprintf(
        "{\"error\": \"Processing error: %s\", \"code\": %d}\n", err, errCode)
    return &result
}

func hashMethod(src string, option HandlerParamsReader) string {
    return fmt.Sprintf("%x", sha256.Sum256([]byte(src)))
}
// handlers

func sendAuthRequest(
        cmd *ClientCmd,
        context* helpers.ConnectionContext,
        option HandlerParamsReader) (*ServerCmd, error) {
    // Send server random key for auth
    rand := helpers.NewSystemRandom()
    key := rand.CreatePassword(option.GetDefaultKeySize())
    contextData := ContextNewData{auth: false, data: key}
    answer := ServerCmd{
        contextUpdater: &contextData,
        baseCmd: baseCmd{Data: key, Target: CmdAuthRequest}}
    return &answer, nil
}

func sendAuthResult(
        cmd *ClientCmd,
        context* helpers.ConnectionContext,
        option HandlerParamsReader) (*ServerCmd, error) {
    // Check client hash
    serverKey := context.GetTmpData()
    var err error
    var result *ServerCmd
    if len(serverKey) == option.GetDefaultKeySize() {
        requestData := (*cmd).Data
        if len(requestData) > 0 {
            if clientParts := strings.Split(requestData, ":"); len(clientParts) == 2 {
                // clientParts[0] - hash from client clientParts[1] - client "salt"
                line := fmt.Sprintf(
                    "%s%s%s",
                    // main key
                    option.GetSecretKey(),
                    // client key
                    clientParts[1],
                    // server key
                    serverKey)

                if hashMethod(line, option) == clientParts[0] {
                    rand := helpers.NewSystemRandom()
                    keySize, node := option.GetCidConstructorData()
                    contextData := ContextNewData{auth: true}
                    // offer new cid
                    answer := ServerCmd{
                        contextUpdater: &contextData,
                        baseCmd: baseCmd{Cid: rand.CreateCid(keySize, node), Target: CmdClientData}}
                    result = &answer

                } else {
                    err = errors.New("Auth failed!")
                }
            } else {
                err = errors.New("Client data format error.")
            }
        } else {
            err = errors.New("Client data not found.")
        }
    } else {
        // incorrect
        err = errors.New("Connection without auth request?")
    }
    return result, err
}

// Update context client data and send ready
func sendReady(
        cmd *ClientCmd,
        context* helpers.ConnectionContext,
        option HandlerParamsReader) (*ServerCmd, error) {
    var err error
    var result *ServerCmd
    infoData := []byte((*cmd).Data)
    if info, loadErr := NewClientInfo(&infoData); loadErr != nil {
        err = loadErr
    } else {
        switch info.Group {
            case GroupClientService, GroupClientServer: {
                cid := (*cmd).Cid
                contextData := ContextNewData{
                    group: info.Group, cid: cid}
                var target int
                switch info.Group {
                    case GroupClientServer: target = CmdPublicMethodsRegistration
                    default: target = CmdWaitCommand
                }
                answer := ServerCmd{
                    contextUpdater: &contextData,
                    baseCmd: baseCmd{Target: target, Cid: cid}}
                result = &answer

            }
            default: err = errors.New("Not accepted client group.")
        }
    }
    return result, err
}

func (cmd *ClientCmd) GetFastHandler() (bool, CmdHandler) {
    // Select fast handler for shot operation
    switch (*cmd).Target {
        case CmdAuthRequest: return true, sendAuthRequest
        case CmdAuth: return true, sendAuthResult
        case CmdClientData: return true, sendReady
        default: return false, nil
    }
}

func (cmd *ClientCmd) RequiredAuth() bool {
    switch (*cmd).Target {
        case CmdExit, CmdAuthRequest, CmdAuth: return false
        default: return true
    }
}

func (cmd *ClientCmd) RequiredGroup() bool {
    switch (*cmd).Target {
        case CmdExit, CmdAuthRequest, CmdAuth, CmdClientData: return false
        default: return true
    }
}

// main handlers
type CoreMsgHandler func(
    msg *CoreMsg,
    serverBusyAccounting *helpers.ServerBusyAccounting,
    serverMethods *helpers.ServerMethods) ([]*ServerCmd, error)

func stateUpdateHandler(
        msg *CoreMsg,
        serverBusyAccounting *helpers.ServerBusyAccounting,
        serverMethods *helpers.ServerMethods) ([]*ServerCmd, error) {
    //
    var err error
    cid := (*msg).Cid
    serverBusyAccounting.SetBusy(cid, msg.GetStatus() == ClientProcStatusBusy)
    cmd := NewServerCmd(CmdServerStatus, cid)
    return []*ServerCmd{cmd}, err
}

func registrationPublicMethods(
        msg *CoreMsg,
        serverBusyAccounting *helpers.ServerBusyAccounting,
        serverMethods *helpers.ServerMethods) ([]*ServerCmd, error) {
    //
    var err error
    var cmd *ServerCmd
    cid := (*msg).Cid
    data := (*msg).Data
    if errReg := serverMethods.FillFromMsgData(cid, &data); errReg != nil {
        rllogger.Outputf(
            rllogger.LogError,
            "client %s failed methods registration with error: %s",
            cid, errReg)
        err = errReg
        cmd = NewServerExitCmd()
    } else {
        rllogger.Outputf(rllogger.LogInfo, "methods of server %s: %s", cid, data)
        cmd = NewServerCmd(CmdWaitCommand, cid)
    }
    return []*ServerCmd{cmd}, err
}

func sendCallMethod(
        msg *CoreMsg,
        serverBusyAccounting *helpers.ServerBusyAccounting,
        serverMethods *helpers.ServerMethods) ([]*ServerCmd, error) {
    //
    var err error
    var cmd, servCmd *ServerCmd
    data := string((*msg).Data)
    cid := (*msg).Cid
    method := (*msg).Method
    if serverMethods.IsPublic(method) {
        if freeCid, exists := serverMethods.SearchFree(method, serverBusyAccounting); exists {
            cmd = NewServerCmd(CmdOk, cid)
            servCmd = NewServerDataCmd(CmdCallMethod, freeCid, &data)
        } else {
            cmd = NewServerCmd(CmdWaitFree, cid)
        }
    } else {
        cmd = NewServerCmd(CmdProblem, cid)
        err = errors.New(fmt.Sprintf("Method '%s' not found", method))
    }
    return []*ServerCmd{cmd, servCmd}, err
}

func notImplTargetHandler(
        msg *CoreMsg,
        serverBusyAccounting *helpers.ServerBusyAccounting,
        serverMethods *helpers.ServerMethods) ([]*ServerCmd, error) {
    //
    return []*ServerCmd{NewServerExitCmd()}, errors.New("Unknown target")
}

func ProcessingServerMsg(
        msg *CoreMsg,
        serverBusyAccounting *helpers.ServerBusyAccounting,
        serverMethods *helpers.ServerMethods) ([]*ServerCmd, error) {
    //
    var handler CoreMsgHandler
    switch (*msg).Target {
        case CmdServerStatus: handler = stateUpdateHandler
        case CmdPublicMethodsRegistration: handler = registrationPublicMethods
        case CmdCallMethod: handler = sendCallMethod
        default: handler = notImplTargetHandler
    }
    return handler(msg, serverBusyAccounting, serverMethods)
}
