package coremethods

import (
	"fmt"
	"roolet/coreprocessing"
)

// return date size in result
func ping(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	var insType int
	var result string
	if cmd, exists := inIns.GetCommand(); exists {
		insType = coreprocessing.TypeInstructionPong
		result = fmt.Sprint(len(cmd.Params.Data) + len(cmd.Params.Json))
	} else {
		insType = coreprocessing.TypeInstructionSkip
	}
	outIns := coreprocessing.NewCoreInstruction(insType)
	if insType == coreprocessing.TypeInstructionPong {
		outIns.SetAnswer(inIns.MakeOkAnswer(result))
	}
	return outIns
}

// check token by client public key
func auth(handler *coreprocessing.Handler, inIns *coreprocessing.CoreInstruction) *coreprocessing.CoreInstruction {
	// TODO
	// client sends in params:
	//  - json: {"key": "<key name>"}
	//  - data: "<base64 token string, use JWT protocol (random data inside)>"
	// Here getting (search on option.KeyDir) client public key by name (in command params)
	// and check it using methods in module "cryptosupport"
	return nil
}

func Setup() {
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionPing, ping)
	coreprocessing.SetupMethod(coreprocessing.TypeInstructionAuth, auth)
}
