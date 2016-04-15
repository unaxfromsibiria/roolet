package connectionserver

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"roolet/connectionsupport"
	"roolet/coreprocessing"
	"roolet/coresupport"
	"roolet/options"
	"roolet/rllogger"
	"roolet/statistic"
	"roolet/transport"
	"sync"
	"time"
)

const (
	ServerStatusOn  = 1
	ServerStatusOff = 2
	// take some default answer
	ServerStatusBusy = 3
)

type statusAcceptedObject struct {
	status           int
	statusChangeLock *sync.RWMutex
}

func (obj *statusAcceptedObject) SetStatus(value int) {
	(*obj).statusChangeLock.Lock()
	defer (*obj).statusChangeLock.Unlock()
	(*obj).status = value
}

func (obj *statusAcceptedObject) GetStatus() int {
	(*obj).statusChangeLock.RLock()
	defer (*obj).statusChangeLock.RUnlock()
	return (*obj).status
}

type ConnectionServer struct {
	statusAcceptedObject
	option                options.SysOption
	stat                  statistic.StatisticUpdater
	connectionDataManager *connectionsupport.ConnectionDataManager
}

func (server *ConnectionServer) Stop() {
	server.SetStatus(ServerStatusOff)
	rllogger.Output(rllogger.LogInfo, "Connection server stopping..")
}

func (server *ConnectionServer) isAcceptForConnection() bool {
	return server.GetStatus() != ServerStatusOff
}

// answer worker
func connectionWriteProcessing(
	connection net.Conn,
	backChannel *chan coreprocessing.CoreInstruction,
	dataManager *connectionsupport.ConnectionDataManager,
	stat statistic.StatisticUpdater,
	label string) {
	//
	wait := true
	msgSize := 0
	var newInstruction coreprocessing.CoreInstruction
	var statGroupName string
	for wait {
		newInstruction = <-*backChannel
		if newInstruction.IsEmpty() {
			rllogger.Outputf(rllogger.LogWarn, "Closed write process for %s empty instruction!", label)
			wait = false
		} else {
			// write
			msgSize = 0
			if answer, exists := newInstruction.GetAnswer(); exists {
				data := answer.DataDump()
				if data != nil {
					line := append(*data, byte('\n'))
					if writen, err := connection.Write(line); err == nil {
						msgSize += writen
					} else {
						rllogger.Outputf(rllogger.LogWarn, "Can't wite answer to %s: %s", label, err)
						stat.AddOneMsg("lost_connection_count")
						// wtf ?
						wait = false
					}
				}
			}
			// and / or command for client
			if answerCmd, exists := newInstruction.GetCommand(); exists && wait {
				data := answerCmd.DataDump()
				if data != nil {
					line := append(*data, byte('\n'))
					if writen, err := connection.Write(line); err == nil {
						msgSize += writen
					} else {
						rllogger.Outputf(rllogger.LogWarn, "Can't wite answer to %s: %s", label, err)
						stat.AddOneMsg("lost_connection_count")
						// wtf ?
						wait = false
					}
				}
			}
			if msgSize > 0 {
				stat.SendMsg("outcome_data_size", msgSize)
				// update state data
				if newInstruction.StateChanges != nil {
					// so simple.. without some visitor for statistic
					if newInstruction.StateChanges.ChangeType == connectionsupport.StateChangesTypeGroup {
						switch newInstruction.StateChanges.ConnectionClientGroup {
						case connectionsupport.GroupConnectionClient:
							statGroupName = "count_connection_client"
						case connectionsupport.GroupConnectionServer:
							statGroupName = "count_connection_server"
						case connectionsupport.GroupConnectionWsClient:
							statGroupName = "count_connection_web"
						}
						stat.AddOneMsg(statGroupName)
					}
					// update data in manager
					dataManager.UpdateState(newInstruction.Cid, newInstruction.StateChanges)
				}
			} else {
				rllogger.Outputf(rllogger.LogWarn, "Empty answer to %s?", label)
			}
			if newInstruction.NeedExit() {
				wait = false
				rllogger.Outputf(rllogger.LogDebug, "Closed write process for %s from instruction.", label)
				err := connection.Close()
				if err != nil {
					// wait = true ??
					rllogger.Outputf(rllogger.LogError, "Close connection %s problem: %s", label, err)
				}
			}
		}
	}
	stat.DelOneMsg(statGroupName)
	close(*backChannel)
}

func (server *ConnectionServer) connectionReadProcessing(
	connection net.Conn,
	workerManager *coresupport.CoreWorkerManager,
	label string) {
	//
	defer connection.Close()
	buffer := bufio.NewReader(connection)
	wait := true
	hasAnswerChannel := false
	connectionData := server.connectionDataManager.NewConnection()
	// TODO: rllogger.LogDebug
	rllogger.Outputf(rllogger.LogInfo, "new connection %s", connectionData.Cid)
	sizeBuffer := (*server).option.BufferSize

	for wait {
		//
		lineData, _, err := buffer.ReadLine()
		if err == nil {
			server.stat.SendMsg("income_data_size", len(lineData))
			if cmd, err := transport.ParseCommand(&lineData); err == nil {
				workerManager.Processing(cmd, server.connectionDataManager, connectionData)
				if !hasAnswerChannel {
					// create
					backChannel := make(chan coreprocessing.CoreInstruction, sizeBuffer)
					// dont like closure, only realy need
					go connectionWriteProcessing(
						connection, &backChannel, (*server).connectionDataManager, server.stat, label)
					workerManager.AppendBackChannel(connectionData, &backChannel)
					hasAnswerChannel = true
				}
			} else {
				server.stat.SendMsg("bad_command_count", 1)
				rllogger.Outputf(rllogger.LogWarn, "connection %s bad command: %s", connectionData.Cid, err)
			}
		} else {
			// has error
			if err == io.EOF {
				// client break connection
				workerManager.BrokenConnection(connectionData)
				rllogger.Outputf(rllogger.LogDebug, "broken connection %s", connectionData.Cid)
			}
			wait = false
		}
	}
	server.stat.SendMsg("connection_count", -1)
	workerManager.RemoveBackChannel(connectionData)
	server.connectionDataManager.RemoveConnection(connectionData.Cid)
	// TODO: rllogger.LogDebug
	rllogger.Outputf(rllogger.LogInfo, "out connection %s", connectionData.Cid)
}

func (server *ConnectionServer) WorkerHandlerConfigure(handler *coreprocessing.Handler) {
	(*handler).StateCheker = (*server).connectionDataManager
}

func (server *ConnectionServer) Start(workerManager *coresupport.CoreWorkerManager) {
	options := (*server).option
	(*server).connectionDataManager = connectionsupport.NewConnectionDataManager(options)
	go server.startListener(workerManager)
}

func (server *ConnectionServer) startListener(workerManager *coresupport.CoreWorkerManager) {
	if server.GetStatus() != ServerStatusOn {
		server.SetStatus(ServerStatusOn)
		options := (*server).option
		socket := options.Socket()
		listener, err := net.Listen("tcp", socket)
		if err == nil {
			defer listener.Close()
			defer (*server).connectionDataManager.Close()
			for server.isAcceptForConnection() {
				newConnection, err := listener.Accept()
				if err != nil {
					rllogger.Outputf(
						rllogger.LogTerminate, "Can't create connection to %s error: %s", socket, err)
					server.stat.SendMsg("lost_connection_count", 1)
				} else {
					clientAddr := fmt.Sprintf("connection:%s", newConnection.RemoteAddr())
					rllogger.Outputf(rllogger.LogDebug, "new %s", clientAddr)
					tcpConnection := newConnection.(*net.TCPConn)
					tcpConnection.SetKeepAlive(true)
					tcpConnection.SetKeepAlivePeriod(
						time.Duration(1+options.StatusCheckPeriod) * time.Second)
					server.stat.SendMsg("connection_count", 1)
					server.connectionReadProcessing(
						newConnection,
						workerManager,
						clientAddr)
				}
			}
		} else {
			rllogger.Outputf(rllogger.LogTerminate, "Can't start server at %s error: %s", socket, err)
		}
	}
}

func NewServer(option options.SysOption, stat *statistic.Statistic) *ConnectionServer {
	stat.AddItem("connection_count", "Client conntection count")
	stat.AddItem("income_data_size", "Income data size")
	stat.AddItem("outcome_data_size", "Outcome data size")
	stat.AddItem("bad_command_count", "Format error command count")
	stat.AddItem("lost_connection_count", "Lost connection count")
	stat.AddItem("auth_request", "Request for auth count")
	stat.AddItem("auth_successfull", "Successfully auth request count")
	stat.AddItem("count_connection_client", "Connection count (service clients)")
	stat.AddItem("count_connection_server", "Connection count (servers)")
	stat.AddItem("count_connection_web", "Connection count (web-socket)")
	//
	server := ConnectionServer{
		statusAcceptedObject: statusAcceptedObject{
			statusChangeLock: new(sync.RWMutex)},
		option: option,
		stat:   stat}
	return &server
}
