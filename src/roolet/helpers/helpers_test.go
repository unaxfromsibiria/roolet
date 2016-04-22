package helpers_test

import (
	"fmt"
	"roolet/helpers"
	"strings"
	"testing"
)

func TestTaskIdGeneratorConcurrency(t *testing.T) {
	generator := helpers.NewTaskIdGenerator()
	var n, wCount, doneCount int
	n = 100000
	wCount = 10
	taskIdSize := len(generator.CreateTaskId())
	doneChannel := make(chan bool, wCount)
	errorChannel := make(chan bool, 1)
	testWorker := func(gen *helpers.TaskIdGenerator, done *chan bool, errorCh *chan bool, size int) {
		for i := 0; i < n; i++ {
			taskId := gen.CreateTaskId()
			if len(taskId) != size {
				*errorCh <- true
			}
		}
		*done <- true
	}
	// run
	for i := 0; i < wCount; i++ {
		go testWorker(generator, &doneChannel, &errorChannel, taskIdSize)
	}
	// wait
	filed := false
	for doneCount < wCount {
		select {
		case done := <-doneChannel:
			{
				if done {
					doneCount++
				}
			}
		case hasErr := <-errorChannel:
			{
				if hasErr && !filed {
					filed = true
				}
			}
		}
	}
	if filed {
		t.Error("Size of task id is not a constant.")
	} else {
		lastTaskId := generator.CreateTaskId()
		lastCounterValue := fmt.Sprintf("%016X", wCount*n+2)
		t.Logf("Last task id: counter(%s) == %s ?", lastTaskId, lastCounterValue)
		parts := strings.Split(lastTaskId, "-")
		if !(len(parts) == 2 && parts[1] == lastCounterValue) {
			t.Error("Unexpected counter value.")
		}
	}
}
