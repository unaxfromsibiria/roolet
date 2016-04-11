package coresupport

import (
	"roolet/connectionsupport"
	"roolet/coreprocessing"
	"roolet/options"
	"roolet/rllogger"
	"roolet/statistic"
	"roolet/transport"
)

func worker(
	index int,
	instructionsChannel *chan coreprocessing.CoreInstruction,
	stopSignalChannel *chan bool,
	outGroups *[]*outChannelGroup,
	handler *coreprocessing.Handler) {
	//
	rllogger.Outputf(rllogger.LogDebug, "Worker %d started...", index)
	active := true

	for active {
		// wait new instruction or finish
		select {
		case <-*stopSignalChannel:
			{
				active = false
			}
		case instruction := <-*instructionsChannel:
			{
				newInstruction := handler.Execute(&instruction)
				// TODO: method for extract 2 int values
				cid := (*newInstruction).Cid
				if connectionData, err := connectionsupport.ExtractConnectionData(cid); err == nil {
					resIndex, id := connectionData.GetResourceIndex(), connectionData.GetId()
					if !(*outGroups)[resIndex].Send(id, newInstruction) {
						rllogger.Outputf(
							rllogger.LogError,
							"Can't send back instruction for %s worker: %d", cid, index)
					}
				} else {
					rllogger.Outputf(
						rllogger.LogError,
						"CID format error! value: %s worker: %d", cid, index)
				}

			}
		}
	}
	handler.Close()
	rllogger.Outputf(rllogger.LogDebug, "Worker %d completed...", index)
}

type outChannelGroup struct {
	connectionsupport.AsyncSafeObject
	channels map[int64]*chan coreprocessing.CoreInstruction
}

func newOutChannelGroup() *outChannelGroup {
	objPtr := connectionsupport.NewAsyncSafeObject()
	group := outChannelGroup{
		AsyncSafeObject: *objPtr,
		channels:        make(map[int64]*chan coreprocessing.CoreInstruction)}
	return &group
}

func (group *outChannelGroup) exists(id int64) bool {
	group.Lock(false)
	defer group.Unlock(false)
	_, exists := (*group).channels[id]
	return exists
}

func (group *outChannelGroup) put(id int64, channelPtr *chan coreprocessing.CoreInstruction) {
	group.Lock(true)
	defer group.Unlock(true)
	(*group).channels[id] = channelPtr
}

func (group *outChannelGroup) Send(id int64, instruction *coreprocessing.CoreInstruction) bool {
	group.Lock(false)
	defer group.Unlock(false)
	channelPtr, exists := (*group).channels[id]
	if exists {
		(*channelPtr) <- (*instruction)
	}
	return exists
}

func (group *outChannelGroup) Append(id int64, channelPtr *chan coreprocessing.CoreInstruction) {
	if !group.exists(id) {
		group.put(id, channelPtr)
	}
}

func (group *outChannelGroup) Remove(id int64) {
	if group.exists(id) {
		group.Lock(true)
		defer group.Unlock(true)
		delete((*group).channels, id)
	}
}

func (group *outChannelGroup) SendExit() {
	group.Lock(true)
	defer group.Unlock(true)
	exitInstruction := coreprocessing.NewExitCoreInstruction()
	for _, outChannelPtr := range (*group).channels {
		(*outChannelPtr) <- (*exitInstruction)
	}
}

type CoreWorkerManager struct {
	options                 options.SysOption
	OutSignalChannel        chan bool
	workerStopSignalChannel chan bool
	instructionsChannel     chan coreprocessing.CoreInstruction
	outChannels             []*outChannelGroup
	// onсe instance everywhere
	methodsDict *coreprocessing.MethodInstructionDict
	statistic   statistic.StatisticUpdater
}

func NewCoreWorkerManager(option options.SysOption, stat *statistic.Statistic) *CoreWorkerManager {
	// setup statistic items
	stat.AddItem("processed", "Processed messages count")
	stat.AddItem("skip_cmd", "Command with skip instruction count")
	manager := CoreWorkerManager{
		OutSignalChannel:        make(chan bool, 1),
		workerStopSignalChannel: make(chan bool, option.Workers),
		instructionsChannel:     make(chan coreprocessing.CoreInstruction, option.BufferSize),
		outChannels:             make([]*outChannelGroup, connectionsupport.GroupCount),
		methodsDict:             coreprocessing.NewMethodInstructionDict(),
		statistic:               stat,
		options:                 option}
	return &manager
}

func (mng *CoreWorkerManager) Start(handlerSetuper coreprocessing.HandlerConfigurator) {
	manager := *mng
	count := manager.options.Workers
	for index := 0; index < count; index++ {
		handler := coreprocessing.NewHandler(index, manager.options, manager.statistic)
		handlerSetuper.WorkerHandlerConfigure(handler)
		go worker(
			index+1,
			&(manager.instructionsChannel),
			&(manager.workerStopSignalChannel),
			&(manager.outChannels),
			handler)
	}
}

func (mng *CoreWorkerManager) Stop() {
	manager := *mng
	count := manager.options.Workers
	for index := 0; index < count; index++ {
		manager.workerStopSignalChannel <- true
	}
	rllogger.Output(rllogger.LogInfo, "Stoping workers..")
	close(manager.instructionsChannel)
	close(manager.workerStopSignalChannel)
	for _, groupPtr := range manager.outChannels {
		if groupPtr != nil {
			groupPtr.SendExit()
		}
	}
	manager.OutSignalChannel <- true
}

func (mng *CoreWorkerManager) Close() {
	close((*mng).OutSignalChannel)
}

func (mng *CoreWorkerManager) BrokenConnection(connData *connectionsupport.ConnectionData) {
	//pass
}

func (mng *CoreWorkerManager) AppendBackChannel(
	connData *connectionsupport.ConnectionData,
	backChannel *chan coreprocessing.CoreInstruction) {
	//
	index := connData.GetResourceIndex()
	if mng.outChannels[index] == nil {
		mng.outChannels[index] = newOutChannelGroup()
	}
	mng.outChannels[index].Append(connData.GetId(), backChannel)
}

func (mng *CoreWorkerManager) RemoveBackChannel(connData *connectionsupport.ConnectionData) {
	index := connData.GetResourceIndex()
	if mng.outChannels[index] != nil {
		mng.outChannels[index].Remove(connData.GetId())
	}
}

func (mng *CoreWorkerManager) Processing(
	cmd *transport.Command,
	connDataManager *connectionsupport.ConnectionDataManager,
	connData *connectionsupport.ConnectionData) {
	// select instraction type by method of command
	instruction := coreprocessing.NewCoreInstructionForMessage(
		mng.methodsDict.Get((*cmd).Method), (*connData).Cid, cmd)

	if instruction.Type == coreprocessing.TypeInstructionSkip {
		mng.statistic.SendMsg("skip_cmd", 1)
	}
	mng.instructionsChannel <- (*instruction)
}
