package connectionserver

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"
    "io"
	"roolet/options"
	"roolet/coresupport"
	"roolet/statistic"
	"roolet/rllogger"
	"roolet/connectionsupport"
	"roolet/transport"
)

const (
	ServerStatusOn = 1
	ServerStatusOff = 2
	// take some default answer
	ServerStatusBusy = 3
)

type statusAcceptedObject struct {
	status int
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
	option options.SysOption
	stat statistic.StatisticUpdater
	connectionDataManager *connectionsupport.ConnectionDataManager
}

func (server *ConnectionServer) Stop() {
	server.SetStatus(ServerStatusOff)
}

func (server *ConnectionServer) isAcceptForConnection() bool {
	return server.GetStatus() != ServerStatusOff
}

// answer worker
func connectionWriteProcessing(
		connection net.Conn,
		backChannel *chan coresupport.CoreInstruction,
		stat statistic.StatisticUpdater,
		label string) {
	// 
	wait := true
	var newInstruction coresupport.CoreInstruction
	for wait {
		newInstruction = <- *backChannel
		if newInstruction.IsEmpty() {
			wait = false
		} else {
			// write
			if answer, exists := newInstruction.GetAnswer(); exists {
				data := answer.DataDump()
				if data == nil {
					rllogger.Outputf(rllogger.LogWarn, "No data for answer to %s?", label)
				} else {
					line := append(*data, byte('\n'))
					if writen, err := connection.Write(line); err == nil {
						stat.SendMsg("outcome_data_size", writen)
					} else {
						rllogger.Outputf(rllogger.LogWarn, "Can't wite answer to %s: %s", label, err)
						stat.SendMsg("lost_connection_count", 1)
						// wtf ?
						wait = false
					}
				}
			} else {
				rllogger.Outputf(rllogger.LogWarn, "Empty answer to %s?", label)
			}
		}
	}
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
	rllogger.Outputf(rllogger.LogDebug, "new connection %s", connectionData.Cid)
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
					backChannel := make(chan coresupport.CoreInstruction, sizeBuffer)
					// dont like closure, only realy need
					go connectionWriteProcessing(connection, &backChannel, server.stat, label)
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
    server.connectionDataManager.RemoveConnection(connectionData.Cid)
}

func (server *ConnectionServer) Start(workerManager *coresupport.CoreWorkerManager) {
	if server.GetStatus() != ServerStatusOn {
		server.SetStatus(ServerStatusOn)
		options := (*server).option
		socket := options.Socket()
		listener, err := net.Listen("tcp", socket)
		if err == nil {
			(*server).connectionDataManager = connectionsupport.NewConnectionDataManager(options)
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
					    time.Duration(1 + options.StatusCheckPeriod) * time.Second)
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
	server := ConnectionServer{
		statusAcceptedObject: statusAcceptedObject{
			statusChangeLock: new(sync.RWMutex)},
		option: option,
		stat: stat}
	return &server
}
