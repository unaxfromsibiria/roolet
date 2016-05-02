package helpers_test

import (
	"fmt"
	"roolet/helpers"
	"strings"
	"testing"
	"time"
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

func TestAsyncStrDictFilling(t *testing.T) {
	dictPtr := helpers.NewAsyncStrDict()
	var n, wCount, doneCount int
	n = 1000
	wCount = 10
	doneChannel := make(chan bool, wCount)
	testWorker := func(index int, dict *helpers.AsyncStrDict, done *chan bool, clear bool) {
		for i := 0; i < n; i++ {
			if clear {
				dict.Delete(fmt.Sprintf("%d-%d", index+1, i+1))
			} else {
				dict.Set(
					fmt.Sprintf("%d-%d", index+1, i+1),
					fmt.Sprintf("%d.%d", index, i))
			}
		}
		*done <- true
	}
	// run workers
	for i := 0; i < wCount; i++ {
		go testWorker(i, dictPtr, &doneChannel, false)
	}
	for doneCount < wCount {
		done := <-doneChannel
		if done {
			doneCount += 1
		}
	}
	s := dictPtr.Size()
	if n*wCount != s {
		t.Errorf("Dict size: %d", s)
	}
	// clear
	doneCount = 0
	for i := 0; i < wCount; i++ {
		go testWorker(i, dictPtr, &doneChannel, true)
	}
	for doneCount < wCount {
		done := <-doneChannel
		if done {
			doneCount += 1
		}
	}
	if dictPtr.Size() != 0 {
		t.Error("Dict size must be 0")
	}
}

func TestAsyncStrDictFillingWithDuplication(t *testing.T) {
	dictPtr := helpers.NewAsyncStrDict()
	var n, wCount, doneCount int
	n = 1000
	wCount = 10
	doneChannel := make(chan bool, wCount)
	testWorker := func(index int, dict *helpers.AsyncStrDict, done *chan bool) {
		for i := 0; i < n; i++ {
			dict.Set(
				fmt.Sprintf("%d-%d", 1, i+1),
				fmt.Sprintf("%d.%d", index, i))
		}
		*done <- true
	}
	// run workers
	for i := 0; i < wCount; i++ {
		go testWorker(i, dictPtr, &doneChannel)
	}
	for doneCount < wCount {
		done := <-doneChannel
		if done {
			doneCount += 1
		}
	}
	s := dictPtr.Size()
	if n != s {
		t.Errorf("Dict size: %d", s)
	}
}

func TestAsyncStrDictGetSetCheck(t *testing.T) {
	dictPtr := helpers.NewAsyncStrDict()
	var n, wCount, doneCount int
	n = 1000
	wCount = 10
	resultChannel := make(chan int, wCount)
	// 1 - done
	// 0 - error
	testWorker := func(read bool, index int, dict *helpers.AsyncStrDict, done *chan int) {
		result := 1
		for i := 0; i < n; i++ {
			if read {
				res := dict.Get(fmt.Sprintf("%d-%d", index+1, i+1))
				if res != nil {
					if *res != fmt.Sprintf("%d.%d", index, i) {
						result = 0
						break
					}
				} else {
					result = 0
					break
				}
			} else {
				dict.Set(
					fmt.Sprintf("%d-%d", index+1, i+1),
					fmt.Sprintf("%d.%d", index, i))
			}
		}
		*done <- result
	}
	// run workers
	for i := 0; i < wCount; i++ {
		go testWorker(false, i, dictPtr, &resultChannel)
	}
	time.Sleep(time.Duration(250) * time.Millisecond)
	for i := 0; i < wCount; i++ {
		go testWorker(true, i, dictPtr, &resultChannel)
	}
	hasError := false
	for doneCount < wCount*2 {
		done := <-resultChannel
		if done == 1 {
			doneCount += 1
		}
		if done == 0 {
			hasError = true
			doneCount += 1
		}
	}
	s := dictPtr.Size()
	if n*wCount != s {
		t.Errorf("Dict size: %d", s)
	}
	if hasError {
		t.Error("Incorrect value")
	}
}

type simpleAnalogDict struct {
	helpers.AsyncSafeObject
	content map[string]string
}

func (dict *simpleAnalogDict) Set(key, value string) {
	dict.Lock(true)
	defer dict.Unlock(true)
	(*dict).content[key] = value
}

// test unsafe set
func TestAsyncStrDictСompareSetSpeed(t *testing.T) {
	dictPtr := helpers.NewAsyncStrDict()
	var n, wCount, doneCount int
	n = 1000
	wCount = 50
	doneChannel := make(chan bool, wCount)
	testWorker := func(index int, dict *helpers.AsyncStrDict, done *chan bool) {
		for i := 0; i < n; i++ {
			dict.SetUnsafe(
				fmt.Sprintf("%d-%d", index+1, i+1),
				fmt.Sprintf("%d.%d", index, i))
		}
		*done <- true
	}
	testWorkerAnalog := func(index int, dict *simpleAnalogDict, done *chan bool) {
		for i := 0; i < n; i++ {
			dict.Set(
				fmt.Sprintf("%d-%d", index+1, i+1),
				fmt.Sprintf("%d.%d", index, i))
		}
		*done <- true
	}
	// run workers
	var startTime time.Time
	startTime = time.Now()
	for i := 0; i < wCount; i++ {
		go testWorker(i, dictPtr, &doneChannel)
	}
	for doneCount < wCount {
		done := <-doneChannel
		if done {
			doneCount += 1
		}
	}
	rtime := float32(time.Since(startTime).Seconds())
	s := dictPtr.Size()
	if n*wCount != s {
		t.Errorf("Dict size: %d", s)
	}
	doneCount = 0
	dictAnalog := simpleAnalogDict{
		content:         make(map[string]string),
		AsyncSafeObject: *(helpers.NewAsyncSafeObject())}

	startTime = time.Now()
	for i := 0; i < wCount; i++ {
		go testWorkerAnalog(i, &dictAnalog, &doneChannel)
	}
	for doneCount < wCount {
		done := <-doneChannel
		if done {
			doneCount += 1
		}
	}
	atime := float32(time.Since(startTime).Seconds())
	t.Logf("real: %f analog: %f\n", rtime, atime)
	if rtime > atime {
		t.Error("broken improvement")
	}
}
