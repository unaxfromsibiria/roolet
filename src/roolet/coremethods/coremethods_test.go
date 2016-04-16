package coremethods_test

import (
	"fmt"
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
			t.Errorf("Answer hasn't error with code: %d", transport.ErrorCodeMethodParamsFormatWrong)
		}
	} else {
		t.Errorf("Empty answer to %s", *cmd)
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
			t.Errorf("Answer hasn't error with code: %d", transport.ErrorCodeMethodParamsFormatWrong)
		}
	} else {
		t.Errorf("Empty answer to %s", *cmd)
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
				"Answer has error with code: %d msg: %s",
				(*answer).Error.Code,
				(*answer).Error.Message)
		} else {
			if outIns.StateChanges != nil {
				if outIns.StateChanges.Status != newStatus {
					t.Errorf("State change delegator has wrong status: %d", outIns.StateChanges.Status)
				}
			} else {
				t.Error("Empty state change delegator")
			}
		}
	} else {
		t.Errorf("Empty answer to %s", *cmd)
	}
}

// ProcRegistration=>
//
func TestRegistrationEmptyParams(t *testing.T) {
	// TODO
}
