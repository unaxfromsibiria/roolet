package coreprocessing

import (
	"roolet/connectionsupport"
	"roolet/options"
	"roolet/rllogger"
	"roolet/statistic"
	"roolet/transport"
)

const (
	TypeInstructionSkip    = 0
	TypeInstructionProblem = 1
	TypeInstructionOk      = 2
	TypeInstructionExit    = 10
	// turnoff it after
	TypeInstructionPing   = 20
	TypeInstructionAuth   = 30
	TypeInstructionPong   = 40
	TypeInstructionStatus = 50
	TypeInstructionReg    = 55
	// method of client
	TypeInstructionExternal = 100
)

type CoreInstruction struct {
	Type         int
	Cid          string
	StateChanges *connectionsupport.StateChanges
	cmd          *transport.Command
	answer       *transport.Answer
}

func NewExitCoreInstruction() *CoreInstruction {
	result := CoreInstruction{Type: TypeInstructionExit}
	return &result
}

func NewCoreInstruction(insType int) *CoreInstruction {
	result := CoreInstruction{Type: insType}
	return &result
}

func NewCoreInstructionForMessage(insType int, cid string, cmd *transport.Command) *CoreInstruction {
	result := CoreInstruction{Type: insType, Cid: cid, cmd: cmd}
	return &result
}

func (instruction *CoreInstruction) MakeErrAnswer(code int, msg string) *transport.Answer {
	answerPtr := instruction.cmd.CreateAnswer()
	(*answerPtr).Error = transport.ErrorDescription{Code: code, Message: msg}
	return answerPtr
}

func (instruction *CoreInstruction) MakeOkAnswer(result string) *transport.Answer {
	answerPtr := instruction.cmd.CreateAnswer()
	(*answerPtr).Result = result
	return answerPtr
}

func (instruction *CoreInstruction) IsEmpty() bool {
	return ((*instruction).cmd == nil &&
		(*instruction).answer == nil &&
		(*instruction).Type == 0)
}

func (instruction *CoreInstruction) NeedExit() bool {
	return ((*instruction).cmd == nil &&
		(*instruction).Type == TypeInstructionExit)
}

func (instruction *CoreInstruction) GetCommand() (*transport.Command, bool) {
	return (*instruction).cmd, (*instruction).cmd != nil
}

func (instruction *CoreInstruction) GetAnswer() (*transport.Answer, bool) {
	return (*instruction).answer, (*instruction).answer != nil
}

func (instruction *CoreInstruction) SetCommand(cmd *transport.Command) {
	(*instruction).cmd = cmd
	if len((*instruction).Cid) <= 0 && len((*cmd).Params.Cid) > 0 {
		(*instruction).Cid = (*cmd).Params.Cid
	}
}

func (instruction *CoreInstruction) SetAnswer(answer *transport.Answer) {
	(*instruction).answer = answer
}

type CidSet struct {
	// wtf! where native set? >_<
	set map[string]struct{}
}

func NewCidSet() *CidSet {
	result := CidSet{set: make(map[string]struct{})}
	return &result
}

func (set *CidSet) Add(cid string) {
	(*set).set[cid] = struct{}{}
}

func (set *CidSet) Size() int {
	return len((*set).set)
}

func (set *CidSet) Remove(cid string) {
	if _, exists := (*set).set[cid]; exists {
		delete((*set).set, cid)
	}
}

func (set *CidSet) Exists(cid string) bool {
	if _, exists := (*set).set[cid]; exists {
		return true
	}
	return false
}

// server manager
type RpcServerManager struct {
	connectionsupport.AsyncSafeObject
	methods map[string]*CidSet
}

func (manager *RpcServerManager) Append(cid string, methods *[]string) {
	manager.Lock(true)
	defer manager.Unlock(true)
	for _, methodName := range *methods {
		if setPtr, exists := (*manager).methods[methodName]; exists {
			setPtr.Add(cid)
		} else {
			cell := NewCidSet()
			cell.Add(cid)
			(*manager).methods[methodName] = cell
		}
	}
}

func (manager *RpcServerManager) GetCidVariants(method string) []string {
	manager.Lock(false)
	defer manager.Unlock(false)
	var result []string
	if set, exists := (*manager).methods[method]; exists {
		size := set.Size()
		if size > 0 {
			result = make([]string, size)
			index := 0
			for cid, _ := range set.set {
				result[index] = cid
				index++
			}	
		}
	}
	return result
}

var onceRpcServerManager = RpcServerManager{
	AsyncSafeObject: *(connectionsupport.NewAsyncSafeObject()),
	methods:         make(map[string]*CidSet)}

func NewRpcServerManager() *RpcServerManager {
	// use as singltone
	return &onceRpcServerManager
}

// main instruction commnad router
type MethodInstructionDict struct {
	connectionsupport.AsyncSafeObject
	content map[string]int
}

var onceMethodInstructionDict MethodInstructionDict = MethodInstructionDict{
	AsyncSafeObject: *(connectionsupport.NewAsyncSafeObject()),
	content: map[string]int{
		"auth":         TypeInstructionAuth,
		"registration": TypeInstructionReg,
		"statusupdate": TypeInstructionStatus,
		"ping":         TypeInstructionPing,
		"quit":         TypeInstructionExit,
		"exit":         TypeInstructionExit}}

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
func (dict *MethodInstructionDict) RegisterClientMethods(methods ...string) int {
	dict.check()
	result := int(len(methods))
	for _, method := range methods {
		if len(method) > 0 && !dict.Exists(method) {
			dict.Lock(true)
			(*dict).content[method] = TypeInstructionExternal
			dict.Unlock(true)
		} else {
			result--
		}
	}
	return result
}

// common methods
func exitHandler(handler *Handler, inInstruction *CoreInstruction) *CoreInstruction {
	outInstruction := NewExitCoreInstruction()
	(*outInstruction).answer = inInstruction.MakeOkAnswer("{\"ok\": true}")
	return outInstruction
}

type InstructionHandlerMethod func(*Handler, *CoreInstruction) *CoreInstruction
type InstructionPostHandlerMethod func(*Handler, *CoreInstruction, *CoreInstruction) []*CoreInstruction

var methods map[int]InstructionHandlerMethod = map[int]InstructionHandlerMethod{
	// TODO: create another one (or more) module for methods
	// TypeInstructionSkip: skipHandler
	// ..
	TypeInstructionExit: exitHandler}

var postMethods map[int]InstructionPostHandlerMethod = map[int]InstructionPostHandlerMethod{}

func SetupMethod(insType int, method InstructionHandlerMethod, postMethod InstructionPostHandlerMethod) {
	methods[insType] = method
	if postMethod != nil {
		postMethods[insType] = postMethod
	}
}

type Handler struct {
	StateCheker connectionsupport.ConnectionStateChecker
	Option      options.SysOption
	Stat        statistic.StatisticUpdater
	worker      int
	methods     *map[int]InstructionHandlerMethod
}

type HandlerConfigurator interface {
	WorkerHandlerConfigure(handler *Handler)
}

func NewHandler(
	workerIndex int,
	option options.SysOption,
	stat statistic.StatisticUpdater) *Handler {
	//
	handler := Handler{
		Option:  option,
		Stat:    stat,
		worker:  workerIndex,
		methods: &methods}

	return &handler
}

func (handler *Handler) Close() {
	// pass
}

func (handler *Handler) Execute(ins *CoreInstruction) []*CoreInstruction {
	var result []*CoreInstruction
	if method, exists := methods[ins.Type]; exists {
		outIns := method(handler, ins)
		(*outIns).Cid = (*ins).Cid
		// post method
		postMethod, exists := postMethods[ins.Type]
		externIns := postMethod(handler, ins, outIns)
		size := 1
		if exists {
			size += len(externIns)
		}
		result = make([]*CoreInstruction, size)
		result[0] = outIns
		if exists {
			for index, methodPtr := range(externIns) {
				result[index + 1] = methodPtr
			}
		}
	} else {
		// send error
		outIns := NewCoreInstruction(TypeInstructionProblem)
		(*outIns).Cid = (*ins).Cid
		(*outIns).answer = ins.MakeErrAnswer(
			transport.ErrorCodeInternalProblem, "Not implimented handler for this type!")
		rllogger.Outputf(
			rllogger.LogError, "Unknown instruction type '%d' from %s", ins.Type, ins.Cid)
		result = make([]*CoreInstruction, 1)
		result[0] = outIns
	}
	// copy cid always
	return result
}
