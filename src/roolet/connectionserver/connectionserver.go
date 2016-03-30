package connectionserver

import (
	"fmt"
	"net"
	"sync"
	"time"
	"roolet/options"
	"roolet/coresupport"
	"roolet/statistic"
	"roolet/rllogger"
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
}

func (server *ConnectionServer) Stop() {
	server.SetStatus(ServerStatusOff)
}

func (server *ConnectionServer) isAcceptForConnection() bool {
	return server.GetStatus() != ServerStatusOff
}

func (server *ConnectionServer) Start(workerManager *coresupport.CoreWorkerManager) {
	if server.GetStatus() != ServerStatusOn {
		server.SetStatus(ServerStatusOn)
		options := (*server).option
		socket := options.Socket()
		listener, err := net.Listen("tcp", socket)
		if err != nil {
	    	rllogger.Outputf(rllogger.LogTerminate, "Can't start server at %s error: %s", socket, err)
		} else {
			defer listener.Close()
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
					// TODO: move to connection handler
				
				}
			}
		}
	}
}


func NewServer(option options.SysOption, stat *statistic.Statistic) *ConnectionServer {
	stat.AddItem("connection_count", "Client conntection count")
	server := ConnectionServer{
		statusAcceptedObject: statusAcceptedObject{
			statusChangeLock: new(sync.RWMutex)},
		option: option,
		stat: stat}
	return &server
}
