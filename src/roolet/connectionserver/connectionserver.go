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

func (server *ConnectionServer) connectionProcessingWorker(
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
    for wait {
    	//
		lineData, _, err := buffer.ReadLine()
		if err == nil {
			server.stat.SendMsg("income_data_size", len(lineData))
			if cmd, err := transport.ParseCommand(&lineData); err == nil {
				workerManager.Processing(cmd, server.connectionDataManager, connectionData)
				// TODO:
				// check and create answer channel
				// registr him in worker manager
				if !hasAnswerChannel {
					// create
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
				} else {
				    clientAddr := fmt.Sprintf("connection:%s", newConnection.RemoteAddr())
					rllogger.Outputf(rllogger.LogDebug, "new %s", clientAddr)
					tcpConnection := newConnection.(*net.TCPConn)
					tcpConnection.SetKeepAlive(true)
					tcpConnection.SetKeepAlivePeriod(
					    time.Duration(1 + options.StatusCheckPeriod) * time.Second)
					server.stat.SendMsg("connection_count", 1)
					server.connectionProcessingWorker(
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
	stat.AddItem("bad_command_count", "Format error command count")
	server := ConnectionServer{
		statusAcceptedObject: statusAcceptedObject{
			statusChangeLock: new(sync.RWMutex)},
		option: option,
		stat: stat}
	return &server
}
