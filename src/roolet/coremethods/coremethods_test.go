package coremethods_test

import (
	"fmt"
	"roolet/connectionsupport"
	"roolet/coremethods"
	"roolet/coreprocessing"
	"roolet/options"
	"roolet/statistic"
	"roolet/transport"
	"testing"
)

// ProcUpdateStatus =>
//
func TestUpdateStatusEmptyParams(t *testing.T) {
	option := options.SysOption{
		Statistic: false}
	stat := statistic.NewStatistic(option)
	handler := coreprocessing.NewHandler(1, option, stat)
	inIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionReg)
	cmd := transport.NewCommand(0, "27d90e5e-0000000000000011-1", "statusupdate", "")
	inIns.SetCommand(cmd)
	outIns := coremethods.ProcUpdateStatus(handler, inIns)
	if answer, exists := outIns.GetAnswer(); exists {
		if (*answer).Error.Code != transport.ErrorCodeMethodParamsFormatWrong {
			t.Errorf("Answer hasn't error with code: %d.", transport.ErrorCodeMethodParamsFormatWrong)
		}
	} else {
		t.Errorf("Empty answer to %s.", *cmd)
	}
}

func TestUpdateStatusValueLimits(t *testing.T) {
	option := options.SysOption{
		Statistic: false}
	stat := statistic.NewStatistic(option)
	handler := coreprocessing.NewHandler(1, option, stat)
	inIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionReg)
	cmd := transport.NewCommand(0, "27d90e5e-0000000000000011-1", "statusupdate", "65537")
	inIns.SetCommand(cmd)
	outIns := coremethods.ProcUpdateStatus(handler, inIns)
	if answer, exists := outIns.GetAnswer(); exists {
		if (*answer).Error.Code == transport.ErrorCodeMethodParamsFormatWrong {
			t.Logf("Right message: %s", (*answer).Error.Message)
		} else {
			t.Errorf("Answer hasn't error with code: %d.", transport.ErrorCodeMethodParamsFormatWrong)
		}
	} else {
		t.Errorf("Empty answer to %s.", *cmd)
	}
}

func TestUpdateStatusChanged(t *testing.T) {
	option := options.SysOption{
		Statistic: false}
	var newStatus uint16 = 1
	stat := statistic.NewStatistic(option)
	handler := coreprocessing.NewHandler(1, option, stat)
	inIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionReg)
	cmd := transport.NewCommand(0, "27d90e5e-0000000000000011-1", "statusupdate", fmt.Sprint(newStatus))
	inIns.SetCommand(cmd)
	outIns := coremethods.ProcUpdateStatus(handler, inIns)
	if answer, exists := outIns.GetAnswer(); exists {
		if (*answer).Error.Code > 0 {
			t.Errorf(
				"Answer has error with code: %d msg: %s.",
				(*answer).Error.Code,
				(*answer).Error.Message)
		} else {
			if outIns.StateChanges != nil {
				if outIns.StateChanges.Status != newStatus {
					t.Errorf("State change delegator has wrong status: %d.", outIns.StateChanges.Status)
				}
			} else {
				t.Error("Empty state change delegator.")
			}
		}
	} else {
		t.Errorf("Empty answer to %s.", *cmd)
	}
}

// ProcRegistration=>
//
func TestRegistrationEmptyParams(t *testing.T) {
	option := options.SysOption{
		Statistic: false}
	stat := statistic.NewStatistic(option)
	handler := coreprocessing.NewHandler(1, option, stat)
	inIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionReg)
	cmd := transport.NewCommand(0, "27d90e5e-0000000000000011-1", "registration", "")
	// expected methods as list of string
	(*cmd).Params.Json = "{\"methods\": \"none\", \"group\": 1}"
	inIns.SetCommand(cmd)
	outIns := coremethods.ProcRegistration(handler, inIns)
	if answer, exists := outIns.GetAnswer(); exists {
		err := (*answer).Error.Code
		if err > 0 {
			t.Logf("Answer has error, %s.", (*answer).Error)
			if err != transport.ErrorCodeMethodParamsFormatWrong {
				t.Error("Unexpected error code: %d.", err)
			}
		} else {
			t.Error("This answer must have error.")
		}
	} else {
		t.Errorf("Empty answer to %s.", *cmd)
	}
}

// implimented interface ConnectionStateChecker for tests usage
type forTestConnectionStateCheck struct {
	Auth bool
}

func (checker *forTestConnectionStateCheck) ClientInGroup(cid string, group int) bool {
	return true
}

func (checker *forTestConnectionStateCheck) ClientBusy(cid string) bool {
	return false
}

func (checker *forTestConnectionStateCheck) CheckStorageExists(index int) bool {
	return true
}

func (checker *forTestConnectionStateCheck) IsAuth(cid string) bool {
	return (*checker).Auth
}

func TestRegistrationAuthFiled(t *testing.T) {
	option := options.SysOption{
		Statistic: false}
	stat := statistic.NewStatistic(option)
	handler := coreprocessing.NewHandler(1, option, stat)
	cheker := forTestConnectionStateCheck{}
	handler.StateCheker = &cheker
	inIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionReg)
	cmd := transport.NewCommand(0, "27d90e5e-0000000000000011-1", "registration", "")
	(*cmd).Params.Json = "{\"methods\": [\"none\"], \"group\": 1}"
	inIns.SetCommand(cmd)
	outIns := coremethods.ProcRegistration(handler, inIns)
	if answer, exists := outIns.GetAnswer(); exists {
		err := (*answer).Error.Code
		if err > 0 {
			t.Logf("Answer has error, %s.", (*answer).Error)
			if err != transport.ErrorCodeAccessDenied {
				t.Error("Unexpected error code: %d.", err)
			}
		} else {
			t.Error("This answer must have error.")
		}
	} else {
		t.Errorf("Empty answer to %s.", *cmd)
	}
}

func TestRegistrationUnknownGroup(t *testing.T) {
	option := options.SysOption{
		Statistic: false}
	stat := statistic.NewStatistic(option)
	handler := coreprocessing.NewHandler(1, option, stat)
	cheker := forTestConnectionStateCheck{Auth: true}
	handler.StateCheker = &cheker
	inIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionReg)
	cmd := transport.NewCommand(0, "27d90e5e-0000000000000011-1", "registration", "")
	(*cmd).Params.Json = "{\"methods\": [\"none\"], \"group\": 4}"
	inIns.SetCommand(cmd)
	outIns := coremethods.ProcRegistration(handler, inIns)
	if answer, exists := outIns.GetAnswer(); exists {
		err := (*answer).Error.Code
		if err > 0 {
			t.Logf("Answer has error, %s.", (*answer).Error)
			if err != transport.ErrorCodeUnexpectedValue {
				t.Error("Unexpected error code: %d.", err)
			}
		} else {
			t.Error("This answer must have error.")
		}
	} else {
		t.Errorf("Empty answer to %s.", *cmd)
	}
}

func TestRegistratioAddGroupClient(t *testing.T) {
	option := options.SysOption{
		Statistic: false}
	stat := statistic.NewStatistic(option)
	handler := coreprocessing.NewHandler(1, option, stat)
	cheker := forTestConnectionStateCheck{Auth: true}
	handler.StateCheker = &cheker
	inIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionReg)
	cmd := transport.NewCommand(0, "27d90e5e-0000000000000011-1", "registration", "")
	(*cmd).Params.Json = fmt.Sprintf(
		"{\"methods\": [\"none\"], \"group\": %d}", connectionsupport.GroupConnectionClient)
	inIns.SetCommand(cmd)
	outIns := coremethods.ProcRegistration(handler, inIns)
	if answer, exists := outIns.GetAnswer(); exists {
		err := (*answer).Error.Code
		if err > 0 {
			t.Error("Answer with problem, %s", (*answer).Error)
		} else {
			t.Log("TODO")
		}
	} else {
		t.Errorf("Empty answer to %s.", *cmd)
	}
}

func TestRegistratioAddGroupServer(t *testing.T) {
	option := options.SysOption{
		Statistic: false}
	stat := statistic.NewStatistic(option)
	handler := coreprocessing.NewHandler(1, option, stat)
	cheker := forTestConnectionStateCheck{Auth: true}
	handler.StateCheker = &cheker
	inIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionReg)
	cmd := transport.NewCommand(0, "27d90e5e-0000000000000011-1", "registration", "")
	(*cmd).Params.Json = fmt.Sprintf(
		"{\"methods\": [\"none\"], \"group\": %d}", connectionsupport.GroupConnectionServer)
	inIns.SetCommand(cmd)
	outIns := coremethods.ProcRegistration(handler, inIns)
	if answer, exists := outIns.GetAnswer(); exists {
		err := (*answer).Error.Code
		if err > 0 {
			t.Error("Answer with problem, %s", (*answer).Error)
		} else {
			t.Log("TODO")
		}
	} else {
		t.Errorf("Empty answer to %s.", *cmd)
	}
}
