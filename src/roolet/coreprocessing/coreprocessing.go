package coreprocessing

import (
	"roolet/connectionsupport"
	"roolet/options"
	"roolet/rllogger"
	"roolet/statistic"
	"roolet/transport"
)

const (
	TypeInstructionSkip = 0
	TypeInstructionExit = 10
	// turnoff it after
	TypeInstructionPing = 20
	TypeInstructionAuth = 30
	// method of client
	TypeInstructionExternal = 100
)

type CoreInstruction struct {
	Type int
	Cid string
	cmd *transport.Command
	answer *transport.Answer
}

func NewExitcCoreInstruction() *CoreInstruction {
	result := CoreInstruction{Type: TypeInstructionExit}
	return &result
}


func NewCoreInstructionForMessage(insType int, cid string, cmd *transport.Command) *CoreInstruction {
	result := CoreInstruction{Type: TypeInstructionExit}
	return &result
}

func (instruction *CoreInstruction) IsEmpty() bool {
	return (
		(*instruction).cmd == nil &&
		(*instruction).answer == nil  &&
		(*instruction).Type == 0)
}

func (instruction *CoreInstruction) NeedExit() bool {
	return (
		(*instruction).cmd == nil &&
		(*instruction).Type == TypeInstructionExit)
}

func (instruction *CoreInstruction) GetCommand() (*transport.Command, bool) {
	return (*instruction).cmd, (*instruction).cmd == nil
}

func (instruction *CoreInstruction) GetAnswer() (*transport.Answer, bool) {
	return (*instruction).answer, (*instruction).answer == nil
}

type MethodInstructionDict struct {
	connectionsupport.AsyncSafeObject
	content map[string]int
}

var onceMethodInstructionDict MethodInstructionDict = MethodInstructionDict{
	AsyncSafeObject: *(connectionsupport.NewAsyncSafeObject()),
	content: map[string]int{
		"auth": TypeInstructionAuth,
		"ping": TypeInstructionPing,
		"quit": TypeInstructionExit,
		"exit": TypeInstructionExit}}

func NewMethodInstructionDict() *MethodInstructionDict {
	// use like singleton
	return &onceMethodInstructionDict
}

func (dict *MethodInstructionDict) check() {
	if len((*dict).content) <= 0 {
		// use once instance only!
		rllogger.Output(rllogger.LogTerminate, "Use NewMethodInstructionDict()")
	}
}

func (dict *MethodInstructionDict) Get(method string) int {
	dict.check()
	dict.Lock(false)
	defer dict.Unlock(false)
	if value, exists := (*dict).content[method]; exists {
		return value
	} else {
		return TypeInstructionSkip
	}
}

func (dict *MethodInstructionDict) Exists(method string) bool {
	dict.Lock(false)
	defer dict.Unlock(false)
	_, exists := (*dict).content[method]
	return exists
}

// rpc methods name must put here
func (dict *MethodInstructionDict) RegisterClientMethods(methods ...string) {
	dict.check()
	for _, method := range methods {
		if len(method) > 0 && !dict.Exists(method) {
			dict.Lock(true)
			(*dict).content[method] = TypeInstructionExternal
			dict.Unlock(true)
		}
	}
}

type InstructionHandlerMethod func(*Handler, *CoreInstruction) *CoreInstruction

var methods map[int]InstructionHandlerMethod = map[int]InstructionHandlerMethod{
	// TODO: create another one (or more) module for methods
	// TypeInstructionSkip: skipHandler
	// ..
}

type Handler struct {
	worker int
	option options.SysOption
	stat statistic.StatisticUpdater
	methods *map[int]InstructionHandlerMethod
}

func NewHandler(
		workerIndex int,
		option options.SysOption,
		stat statistic.StatisticUpdater) *Handler {
	//
	handler := Handler{
		worker: workerIndex,
		option: option,
		methods: &methods,
		stat: stat}
	return &handler
}

func (handler *Handler) Close() {
	// pass
}

func (handler *Handler) Execute(ins *CoreInstruction) *CoreInstruction{
	// TODO: delegate to handler.methods
	return NewExitcCoreInstruction()
}
