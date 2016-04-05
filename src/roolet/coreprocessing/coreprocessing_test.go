package coreprocessing_test

import (
	"testing"
	"roolet/coreprocessing"
)


func TestSingleMethodsDict(t *testing.T) {
	dict := coreprocessing.NewMethodInstructionDict()
	dict.RegisterClientMethods("test1", "test2")
	innerFunc := func() {
		newDict := coreprocessing.NewMethodInstructionDict()
		t.Logf("%p == %p ?", dict, newDict)
		if !newDict.Exists("test1") || !newDict.Exists("test2") {
			t.Error("Various instance!")
		}
	}
	innerFunc()
}
