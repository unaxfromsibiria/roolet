package coresupport

import (
	"roolet/options"
	"roolet/statistic"
	"roolet/rllogger"
	"roolet/transport"
	"roolet/connectionsupport"
	"roolet/coreprocessing"
)

type CoreInstruction struct {
	Type int
	cid string
	cmd *transport.Command
	answer *transport.Answer
}

func (instruction *CoreInstruction) IsEmpty() bool {
	return (*instruction).cmd == nil && (*instruction).answer == nil
}

func (instruction *CoreInstruction) GetCommand() (*transport.Command, bool) {
	return (*instruction).cmd, (*instruction).cmd == nil
}

func (instruction *CoreInstruction) GetAnswer() (*transport.Answer, bool) {
	return (*instruction).answer, (*instruction).answer == nil
}

func worker(
		index int,
		instructionsChannel *chan CoreInstruction,
		stopSignalChannel *chan bool,
		option options.SysOption,
		stat statistic.StatisticUpdater) {
	//
	rllogger.Outputf(rllogger.LogDebug, "Worker %d started...", index)
	active := true

	for active {
		// wait new instruction or finish
		select {
			case instruction := <- *instructionsChannel: {
				if instruction.Type != coreprocessing.TypeInstructionSkip {
					// TODO: 
				} 
			}
			case <- *stopSignalChannel: {
				active = false
			}
		}
	}
	rllogger.Outputf(rllogger.LogDebug, "Worker %d completed...", index)
}

type outChannelGroup struct {
	connectionsupport.AsyncSafeObject
	channels map[int64]*chan CoreInstruction
}

func newOutChannelGroup() *outChannelGroup {
	objPtr := connectionsupport.NewAsyncSafeObject()
	group := outChannelGroup{
		AsyncSafeObject: *objPtr,
		channels: make(map[int64]*chan CoreInstruction)}
	return &group
}

func (group *outChannelGroup) exists(id int64) bool {
	group.Lock(false)
	defer group.Unlock(false)
	_, exists := (*group).channels[id]
	return exists
}

func (group *outChannelGroup) put(id int64, channelPtr *chan CoreInstruction) {
	group.Lock(true)
	defer group.Unlock(true)
	(*group).channels[id] = channelPtr
}

func (group *outChannelGroup) Append(id int64, channelPtr *chan CoreInstruction) {
	if !group.exists(id) {
		group.put(id, channelPtr)
	} 
}

type CoreWorkerManager struct {
	options options.SysOption
	OutSignalChannel chan bool
	workerStopSignalChannel chan bool
	instructionsChannel chan CoreInstruction
	outChannels []*outChannelGroup
	// onсe instance everywhere
	methodsDict *coreprocessing.MethodInstructionDict
	statistic statistic.StatisticUpdater
}

func NewCoreWorkerManager(option options.SysOption, stat *statistic.Statistic) *CoreWorkerManager {
	// setup statistic items
	stat.AddItem("processed", "Processed messages count")
	stat.AddItem("skip_cmd", "Command with skip instruction count")
	manager := CoreWorkerManager{
		OutSignalChannel: make(chan bool, 1),
		workerStopSignalChannel: make(chan bool, option.Workers),
		instructionsChannel: make(chan CoreInstruction, option.BufferSize),
		outChannels: make([]*outChannelGroup, connectionsupport.GroupCount),
		methodsDict: coreprocessing.NewMethodInstructionDict(),
		statistic: stat,
		options: option}
	return &manager
}

func (mng *CoreWorkerManager) Start() {
	manager := *mng
	count := manager.options.Workers
	for index := 0; index < count; index ++ {
		go worker(
			index + 1,
			&(manager.instructionsChannel),
			&(manager.workerStopSignalChannel),
			manager.options,
			manager.statistic)
	}
}

func (mng *CoreWorkerManager) Stop() {
	manager := *mng
	count := manager.options.Workers
	for index := 0; index < count; index ++ {
		mng.workerStopSignalChannel <- true
	}
}

func (mng *CoreWorkerManager) Close() {
	close((*mng).OutSignalChannel)
}

func (mng *CoreWorkerManager) BrokenConnection(connData *connectionsupport.ConnectionData) {
	//pass
}

func (mng *CoreWorkerManager) AppendBackChannel(
		connData *connectionsupport.ConnectionData,
		backChannel *chan CoreInstruction) {
	//
	mng.outChannels[connData.GetResourceIndex()].Append(connData.GetId(), backChannel)
}

func (mng *CoreWorkerManager) Processing(
		cmd *transport.Command,
		connDataManager *connectionsupport.ConnectionDataManager,
		connData *connectionsupport.ConnectionData) {
	// select instraction type by method of command
	instruction := CoreInstruction{cmd: cmd, cid: (*connData).Cid}
	instruction.Type = mng.methodsDict.Get((*cmd).Method)
	if instruction.Type == coreprocessing.TypeInstructionSkip {
		mng.statistic.SendMsg("skip_cmd", 1)	
	}
	mng.instructionsChannel <- instruction
}
