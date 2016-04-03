package coresupport

import (
	"roolet/options"
	"roolet/statistic"
	"roolet/rllogger"
	"roolet/transport"
	"roolet/connectionsupport"
)

const (
	TypeCoreInstructionSkip = 0
	TypeCoreInstructionExit = 10
)

type CoreInstruction struct {
	Type int
	cmd *transport.Command
	answer *transport.Answer
	dataManager *connectionsupport.ConnectionDataManager
	connectionInfo *connectionsupport.ConnectionData
}

func (instruction *CoreInstruction) IsEmpty() bool {
	return (*instruction).cmd == nil && (*instruction).answer == nil
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
				if instruction.Type != TypeCoreInstructionSkip {
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

type CoreWorkerManager struct {
	options options.SysOption
	OutSignalChannel chan bool
	workerStopSignalChannel chan bool
	instructionsChannel chan CoreInstruction
	statistic statistic.StatisticUpdater
}

func NewCoreWorkerManager(option options.SysOption, stat *statistic.Statistic) *CoreWorkerManager {
	// setup statistic items
	stat.AddItem("processed", "Processed messages count")
	manager := CoreWorkerManager{
		OutSignalChannel: make(chan bool, 1),
		workerStopSignalChannel: make(chan bool, option.Workers),
		instructionsChannel: make(chan CoreInstruction, option.BufferSize),
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

func (mng *CoreWorkerManager) Processing(
		cmd *transport.Command,
		connDataManager *connectionsupport.ConnectionDataManager,
		connData *connectionsupport.ConnectionData) {
	// select instraction typy by method of command
}