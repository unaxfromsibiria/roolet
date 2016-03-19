package rlserver

import (
    "sync"
    "roolet/options"
    "roolet/helpers"
    "roolet/rllogger"
    "roolet/protocol"
    //"roolet/connectionsupport"
    "bufio"
    "fmt"
    "io"
    "net"
    "net/http"
    "syscall"
    "os"
    "os/signal"
    "time"
    "github.com/gorilla/websocket"
)

type StatValueType float32

const (
    oneInStatStep = 1000000
    waitAfterReturnRequest = 50 * time.Microsecond
    // statistic items
    StatIncomeDataSize = 1
    StatIncomeDataStreamVolume = 2
    StatBackRequestToBuffer = 3
    StatConnectionCount = 4
    StatOutcomeDataSize = 5
    StatWorkerRunTime = 6
    StatWorkerLoad = 7
    StatBusyCount = 8
)

// statistic
func statItemName(itemPosition int) string {
    switch itemPosition {
        case StatIncomeDataSize: return "Income data size (b)"
        case StatIncomeDataStreamVolume: return "Income data stream volume (b/sec)"
        case StatBackRequestToBuffer: return "Returned to buffer requests count"
        case StatConnectionCount: return "Connections"
        case StatOutcomeDataSize: return "Outcome data size (b)"
        case StatWorkerRunTime: return "In worker sum time of handler (sec)"
        case StatWorkerLoad: return "Worker load (% of full time)"
        case StatBusyCount: return "Method busy hits"
        default: return "unknown"
    }
}

type StatObjectMeter interface {
    GetSize() float32
}

type connectionInfo struct {
    helpers.CoroutineActiveResource
    label string
}

func newConnectionInfo(label string) *connectionInfo {
    result := connectionInfo{
        *(helpers.NewCoroutineActiveResource()),
        label}
    return &result
}

func (info *connectionInfo) GetLabel() string {
    return info.label
}

type AnswerDispatcher struct {
    helpers.CoroutineActiveResource
    channels map[string]*chan protocol.ServerCmd
}

func (dispatcher *AnswerDispatcher) checkUsedChannel(cid string, channel *chan protocol.ServerCmd) {
    (*dispatcher).RLockChange()
    defer (*dispatcher).RUnLockChange()
    if usedChannel, exists := dispatcher.channels[cid]; exists {
        if channel != usedChannel {
            rllogger.Outputf(
                rllogger.LogWarn,
                "New channel %p change used %p for client id %s",
                channel, usedChannel, cid)
        }
    }
}

func (dispatcher *AnswerDispatcher) AddChannel(cid string, channel *chan protocol.ServerCmd) {
    dispatcher.checkUsedChannel(cid, channel)
    dispatcher.LockChange()
    defer dispatcher.UnLockChange()
    (*dispatcher).channels[cid] = channel
}

func (dispatcher *AnswerDispatcher) RemoveChannel(cid string, closeChannel bool) bool {
    (*dispatcher).RLockChange()
    defer (*dispatcher).RUnLockChange()
    result := false
    if channelPtr, exists := dispatcher.channels[cid]; exists {
        result = exists
        if closeChannel {
            close(*channelPtr)
        }
        delete((*dispatcher).channels, cid)
    }
    return result
}

func (dispatcher *AnswerDispatcher) Put(cid string, cmd *protocol.ServerCmd) {
    dispatcher.LockChange()
    defer dispatcher.UnLockChange()
    if channelPtr, exists := dispatcher.channels[cid]; exists {
        *channelPtr <- *cmd
    } else {
        rllogger.Outputf(rllogger.LogError, "Channel for answer to %s is lost!", cid)
    }
}

func (dispatcher *AnswerDispatcher) PutAll(cmd *protocol.ServerCmd) {
    dispatcher.LockChange()
    defer dispatcher.UnLockChange()
    for cid, channelPtr := range (*dispatcher).channels {
        cmdPtr := cmd.Copy(cid)
        *channelPtr <- *cmdPtr
    }
}

func (dispatcher *AnswerDispatcher) HasClients() bool {
    dispatcher.RLockChange()
    defer dispatcher.RUnLockChange()
    return len((*dispatcher).channels) > 0
}

func (dispatcher *AnswerDispatcher) CloseAll() {
    dispatcher.LockChange()
    defer dispatcher.UnLockChange()
    for cid, channelPtr := range (*dispatcher).channels {
        rllogger.Outputf(rllogger.LogDebug, "close channel %s", cid)
        close(*channelPtr)
        delete((*dispatcher).channels, cid)
    }
}

func NewAnswerDispatcher() *AnswerDispatcher {
    result := AnswerDispatcher{
        channels: make(map[string]*chan protocol.ServerCmd),
        CoroutineActiveResource: *(helpers.NewCoroutineActiveResource())}
    return &result
}

type RlStatistic struct {
    helpers.CoroutineActiveResource
    used bool
    items map[int]StatValueType
    workerCount int
}

func NewRlStatistic() *RlStatistic {
    statistic := RlStatistic{
        CoroutineActiveResource: *(helpers.NewCoroutineActiveResource())}
    return &statistic
}

func (stat *RlStatistic) Run(server *RlServer) {
    options := (*server).GetOption()
    (*stat).workerCount = options.Workers
    (*stat).used = options.Statistic
    (*stat).SetActive(options.Statistic)
    statItems := make(map[int]StatValueType)

    (*stat).items = statItems
    if options.Statistic {
        // print logs
        go func() {
            startTime := time.Now()
            for server.IsActive() {
                time.Sleep(time.Duration(server.option.StatisticCheckTime) * time.Second)
                var lines []string;
                for item, value := range (*stat).items {
                    lines = append(lines, fmt.Sprintf("%s : %f", statItemName(item), value))
                }
                rllogger.OutputLines(rllogger.LogInfo, "statistic", &lines)
                (*stat).update(float32(time.Since(startTime).Seconds()))
            }
            (*stat).SetActive(false)
        }()
    }
}

func (stat *RlStatistic) Finish() {
    if (*stat).used {
        // wait
        wait := 0
        for (*stat).IsActive() {
            time.Sleep(1 * time.Second)
            wait ++
        }
        rllogger.Outputf(
            rllogger.LogInfo,
            "statistic finished after %d sec.",
            wait)
    }
}

func (stat *RlStatistic) update(runtime float32) {
    stat.LockChange()
    defer stat.UnLockChange()
    (*stat).items[StatIncomeDataStreamVolume] = StatValueType(
        float32((*stat).items[StatIncomeDataSize]) / runtime)
    (*stat).items[StatWorkerLoad] = StatValueType(
        float32((*stat).items[StatWorkerRunTime]) / (float32((*stat).workerCount) * runtime) * 100.0)
}

func (stat *RlStatistic) Append(item int, value interface{}) {
    var statValue StatValueType
    switch val := value.(type) {
        case StatObjectMeter: statValue = StatValueType(val.GetSize())
        case int: statValue = StatValueType(val)
        case int32: statValue = StatValueType(val)
        case int64: statValue = StatValueType(val)
        case uint32: statValue = StatValueType(val)
        case uint64: statValue = StatValueType(val)
        case uintptr: statValue = StatValueType(val)
        case float32: statValue = StatValueType(val)
        case float64: statValue = StatValueType(val)
        default: rllogger.Outputf(
            rllogger.LogTerminate,
            "Statistic not accept this type: %T", value)
    }
    statValue = statValue / oneInStatStep
    stat.LockChange()
    defer stat.UnLockChange()
    oldValue, exists := (*stat).items[item]
    if exists {
        statValue += oldValue
    }
    (*stat).items[item] = statValue
}

// server
type RlServer struct {
    helpers.CoroutineActiveResource
    option options.SysOption
    connectionCount int
    changeCountLock *sync.RWMutex
}

func (server *RlServer) ChangeConnectionCount(delta int) {
    (*server).changeCountLock.Lock()
    defer (*server).changeCountLock.Unlock()
    (*server).connectionCount += delta
}

func (server *RlServer) HasConnection() bool {
    (*server).changeCountLock.RLock()
    defer (*server).changeCountLock.RUnlock()
    return (*server).connectionCount > 0
}

func (server *RlServer) info() string {
    return fmt.Sprintf("%T:\n%s\n", server.option, server.option)
}

func (server *RlServer) GetOption() options.SysOption {
    return (*server).option
}

func NewServerCreate(optionSrc options.OptionLoder) *RlServer {
   option, err := optionSrc.Load(true)
   if err != nil {
       rllogger.Outputf(rllogger.LogTerminate, "Option load failed: %s", err)
   }
   server := RlServer{
       option: *option,
       changeCountLock: new(sync.RWMutex),
       CoroutineActiveResource: *(helpers.NewCoroutineActiveResource())}

   rllogger.Output(rllogger.LogInfo, server.info())
   return &server
}

func runWsServer(server *RlServer) {
    // TODO TODO TODO >_<
    // client send JSON-RPC

    upgrader := websocket.Upgrader{
	    ReadBufferSize: 1024,
	    WriteBufferSize: 1024,
    }
    httpHandler := func(response http.ResponseWriter, request *http.Request) {
    	_, err := upgrader.Upgrade(response, request, nil)
    	if err != nil {
    		panic(err)
    		return
    	}
    }
    http.HandleFunc("/ws", httpHandler)
    options := (*server).option
    addr := fmt.Sprintf("%s:%d", options.WsAddr, options.WsPort)
	err := http.ListenAndServe(addr, nil)
	if err != nil {
	    panic(fmt.Sprintf("Can't start server at %s error: %s", addr, err))
	}
}

func conntectionWriteAnswer(
        connection net.Conn,
        answerChannel *chan protocol.ServerCmd,
        thisConnectionInfo *connectionInfo,
        statistic *RlStatistic) {
    // async write to connection from answer channel
    active := true
    var answer *string
    //
    clientLabel := thisConnectionInfo.GetLabel()
    rllogger.Outputf(rllogger.LogDebug, " open writer to %s", clientLabel)
    for active {
        answer = nil
        serverCmd := <- *answerChannel
        if answerDataPtr, dumpErr := serverCmd.Dump(); dumpErr != nil {
            rllogger.Outputf(
                rllogger.LogError, "Dump answer error: %s from %s", dumpErr, clientLabel)
            answer = protocol.FastErrorAnswer(dumpErr, protocol.ErrCodeFormatProblem)
        } else {
            answer = answerDataPtr
        }
        active = thisConnectionInfo.IsActive()
        if active && answer != nil {
            writen, err := connection.Write([]byte(*answer))
            if err != nil {
                active = false
                rllogger.Outputf(
                    rllogger.LogError,
                    "Can't send answer to %s error: %s",
                    clientLabel, err)
            } else {
                // if this is last command after channel closing
                active = !serverCmd.IsEmpty()
                statistic.Append(StatOutcomeDataSize, writen)
            }
        }
    }
    rllogger.Outputf(rllogger.LogDebug, " closed writer to %s", clientLabel)
}

func connectionHandler(
        server *RlServer,
        toCoreBuffer *chan protocol.CoreMsg,
        connection net.Conn,
        label string,
        dispatcher *AnswerDispatcher,
        stat *RlStatistic) {
    // server and service connections
    defer connection.Close()
    options := server.GetOption()
	connectionContext := helpers.ConnectionContext{}
	buffer := bufio.NewReader(connection)
    wait := true
    answerWriterActive := false
    var answer *string
    answerBuffer := make(chan protocol.ServerCmd, options.BufferSize)
    localConnectionInfo := newConnectionInfo(label)
    for wait {
        line, _, err := buffer.ReadLine()
        answer = nil
        if err != nil {
            if err == io.EOF {
                // client break connection
                localConnectionInfo.SetActive(false)
                if cid, exists := connectionContext.GetCid(); exists {
                    dispatcher.RemoveChannel(cid, true)
                }
            }
            wait = false
        } else {
            stat.Append(StatIncomeDataSize, len(line))
            cmd, err := protocol.NewClientCmd(&line)
            if err != nil {
                wait = false
                rllogger.Outputf(
                    rllogger.LogWarn,
                    "Incorrect command from %s error: %s",
                    label, err)
                answer = protocol.FastErrorAnswer(err, protocol.ErrCodeFormatProblem)
            } else {
                if cmd.RequiredClose() {
                    wait = false
                    rllogger.Outputf(rllogger.LogInfo, "close command from %s", label)
                } else {
                    // check auth
                    if cmd.RequiredAuth() && !connectionContext.IsAuth() {
                        answer = protocol.FastErrorAnswer(
                            "Access denied!", protocol.ErrCodeAccessDenied)
                    } else {
                        // try get fast handler
                        if exists, handler := cmd.GetFastHandler(); exists {
                            if newAnswer, err := handler(cmd, &connectionContext, options); err != nil {
                                rllogger.Outputf(
                                    rllogger.LogWarn,
                                    "Client: %s target: %d Handler error: %s",
                                    label, cmd.Target, err)
                                answer = protocol.FastErrorAnswer(err, protocol.ErrCodeProtocolProblem)
                            } else {
                                if dataPtr, dumpErr := newAnswer.Dump(); dumpErr != nil {
                                    answer = protocol.FastErrorAnswer(err, protocol.ErrCodeImplementationProblem)
                                } else {
                                    answer = dataPtr
                                    if exists, contextUpdater := newAnswer.GetContextUpdater(); exists {
                                        contextUpdater.Update(&connectionContext)
                                        rllogger.Outputf(
                                            rllogger.LogDebug, "-> %s context updated: %s", label, connectionContext)
                                    }
                                }
                            }
                        } else {
                            clientGroup := connectionContext.GetClientGroup()
                            msg, convertError := protocol.NewCoreMsg(clientGroup, cmd)
                            if convertError != nil {
                                rllogger.Outputf(
                                    rllogger.LogWarn,
                                    "Msg create error: %s from %s",
                                    convertError, label)

                                answer = protocol.FastErrorAnswer(
                                    convertError, protocol.ErrCodeProtocolProblem)
                            } else {
                                *toCoreBuffer <- *msg
                                if !answerWriterActive {
                                    answerWriterActive = true
                                    // this channel to dispatcher
                                    dispatcher.AddChannel((*msg).Cid, &answerBuffer)
                                    // run async answer writer
                                    go conntectionWriteAnswer(
                                        connection,
                                        &answerBuffer,
                                        localConnectionInfo,
                                        stat)
                                }
                            }
                        }
                    }
                }
            }
            // write sync handler answer
            if answer != nil {
                writen, err := connection.Write([]byte(*answer))
                if err != nil {
                    rllogger.Outputf(
                        rllogger.LogError,
                        "Can't send answer to %s error: %s",
                        label, err)
                } else {
                    stat.Append(StatOutcomeDataSize, writen)
                }
            }
        }
    }
    //not need "close(answerBuffer)" dispatcher do it
    stat.Append(StatConnectionCount, -1)
    server.ChangeConnectionCount(-1)
}

func upConnection(
        server *RlServer,
        toCoreBuffer *chan protocol.CoreMsg,
        answerDispatcher *AnswerDispatcher,
        statistic *RlStatistic) {
    // connections
    options := server.GetOption()
    socket := options.Socket()
    listener, err := net.Listen("tcp", socket)
	if err != nil {
	    rllogger.Outputf(
	        rllogger.LogTerminate,
	        "Can't start server at %s error: %s",
	        socket, err)
	}
	defer listener.Close()
	for server.IsActive() {
		newConnection, err := listener.Accept()
		if err != nil {
    	    rllogger.Outputf(
    	        rllogger.LogTerminate,
    	        "Can't start server at %s error: %s",
    	        socket, err)
		} else {
		    clientAddr := fmt.Sprintf("connection:%s", newConnection.RemoteAddr())
		    rllogger.Outputf(rllogger.LogInfo, "new %s", clientAddr)
		    statistic.Append(StatConnectionCount, 1)
		    server.ChangeConnectionCount(1)
		    tcpConnection := newConnection.(*net.TCPConn)
            tcpConnection.SetKeepAlive(true)
            tcpConnection.SetKeepAlivePeriod(
                time.Duration(1 + options.StatusCheckPeriod) * time.Second)
            //
    		go connectionHandler(
    		    server,
    		    toCoreBuffer,
    		    newConnection,
    		    clientAddr,
    		    answerDispatcher,
    		    statistic)
		}
	}
}

// Send command to all client
func serviceBroadcasting(
        server *RlServer,
        answerDispatcher *AnswerDispatcher) {
    // 
    option := server.GetOption()
    cmd := protocol.NewServerCmd(protocol.CmdServerStatus, "")
    debug := rllogger.UseLogDebug()
    for server.IsActive() {
        if answerDispatcher.HasClients() {
            var startTime time.Time
            if debug {
                startTime = time.Now()                
            }
            answerDispatcher.PutAll(cmd)
            if debug {
                rllogger.Outputf(
                    rllogger.LogDebug,
                    "Send status request per %f", float32(time.Since(startTime).Seconds()))
            }
        }
        time.Sleep(time.Duration(option.StatusCheckPeriod) * time.Second);        
    }
}

func coreWorker(
        index int,
        server *RlServer,
        toCoreBuffer *chan protocol.CoreMsg,
        answerDispatcher *AnswerDispatcher,
        serverBusyAccounting *helpers.ServerBusyAccounting,
        serverMethods *helpers.ServerMethods,
        statistic *RlStatistic) {
    //
    rllogger.Outputf(rllogger.LogInfo, "Worker %d started.", index)
    needWorker := true
    option := server.GetOption()
    var startTime time.Time
    for needWorker && server.IsActive() {
        msg := <-(*toCoreBuffer)
        rllogger.Outputf(rllogger.LogDebug, "worker %d -> %s", index, msg)
        if msg.IsEmpty() {
            needWorker = false
            // send to out answer clients connections
        } else {
            if option.CountWorkerTime {
                startTime = time.Now()
            }
            switch msg.Group {
                case protocol.GroupClientWeb: {
                    // TODO: msg from web socket
                }
                case protocol.GroupClientServer, protocol.GroupClientService: {
                    // rpc server client or application client
                    answers, err := protocol.ProcessingMsg(
                        &msg,
                        serverBusyAccounting,
                        serverMethods)

                    for answIndex := 0; answIndex < len(answers); answIndex ++ {
                        answer := answers[answIndex]
                        if answer != nil {
                            if answer.TargetIs(protocol.CmdWaitFree) {
                                statistic.Append(StatBusyCount , 1)
                                rllogger.Outputf(
                                    rllogger.LogDebug,
                                    "Workers busy, msg: %s",
                                    msg)
                            }
                            cid := (*answer).Cid
                            if len(cid) < 1 {
                                cid = msg.Cid
                            }
                            answerDispatcher.Put(cid, answer)
                        }
                    }
                    if err != nil {
                        rllogger.Outputf(
                            rllogger.LogWarn,
                            "Processing error %s at worker %d %s",
                            err, index, msg)
                    }
                }
                default: {
                    // epic fail
                    rllogger.Outputf(
                        rllogger.LogTerminate,
                        "Error! Unknown group! Worker: %d msg: %s",
                        index, msg)
                }
            }
            if option.CountWorkerTime {
                statistic.Append(
                    StatWorkerRunTime,
                    float32(time.Since(startTime).Seconds()))
            }
        }
    }
    rllogger.Outputf(rllogger.LogInfo, "Worker %d finished.", index)
}

func (server RlServer) Run() {
    server.SetActive(true)
    msgToCoreBuffer := make(chan protocol.CoreMsg, server.option.BufferSize)
    statistic := NewRlStatistic()
    statistic.Run(&server)
    answerDispatcher := NewAnswerDispatcher()
    serverBusyAccounting := helpers.NewServerBusyAccounting()
    serverMethods := helpers.NewServerMethods()
    go upConnection(&server, &msgToCoreBuffer, answerDispatcher, statistic)
    //connectionSupport := connectionsupport.NewConnectionAccounting()
    // TODO: use it
    //connectionSupport.Dec()

    for index := 1; index < server.option.Workers + 1; index ++ {
        go coreWorker(
            index,
            &server,
            &msgToCoreBuffer,
            answerDispatcher,
            serverBusyAccounting,
            serverMethods,
            statistic)
    }
    // status service
    go serviceBroadcasting(&server, answerDispatcher)
    // wait sigterm
    signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
    signal.Notify(signalChannel, syscall.SIGTERM)
    for server.IsActive() {
        newSig := <- signalChannel
		if newSig != nil {
		    server.SetActive(false)
		}
    }
    rllogger.Output(rllogger.LogInfo, "stoping server, wait...")
    answerDispatcher.CloseAll()
    close(signalChannel)
    for server.HasConnection() {
        rllogger.Output(rllogger.LogInfo, "wait closing client connection.")
        time.Sleep(1 * time.Second)
    }
    close(msgToCoreBuffer)
    statistic.Finish()
}
