package coremethods_test

import (
	"roolet/coremethods"
	"roolet/coreprocessing"
	"roolet/options"
	"roolet/statistic"
	"testing"
)

func updateStatusEmptyParams(t *testing.T) {
	option := options.SysOption{
		Statistic: false}
	stat := statistic.NewStatistic(option)
	handler := coreprocessing.NewHandler(1, option, stat)
	inIns := coreprocessing.NewCoreInstruction(coreprocessing.TypeInstructionReg)
	// TODO
	coremethods.ProcUpdateStatus(handler, inIns)
}
