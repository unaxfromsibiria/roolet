package coreprocessing

import (
	"roolet/rllogger"
	"roolet/connectionsupport"
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

type MethodInstructionDict struct {
	connectionsupport.AsyncSafeObject
	content map[string]int
}

var onceMethodInstructionDict MethodInstructionDict = MethodInstructionDict{
	AsyncSafeObject: *(connectionsupport.NewAsyncSafeObject()),
	content: map[string]int{
		"auth": TypeInstructionAuth,
		"ping": TypeInstructionPing,
		"exit": TypeInstructionSkip}}

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
