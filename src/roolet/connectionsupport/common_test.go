package connectionsupport_test

import (
    "roolet/connectionsupport"
    "roolet/options"
    "testing"
    "time"
)

func TestConcurrencyCounterUse(t *testing.T) {
	option := options.SysOption{}
	manager := connectionsupport.NewConnectionDataManager(option)
	wCount := 10
	operCount := 1000
	worker := func(m *connectionsupport.ConnectionDataManager, inc bool) {
		for i := 0; i < operCount; i ++ {
			if inc {
				m.Inc()				
			} else {
				m.Dec()
			}
		}
	}
	for index := 0; index < wCount; index ++ {
		go worker(manager, true)
		go worker(manager, false)
	}
	delay := 3.2
	t.Logf("Wait %f sec.", delay)
	time.Sleep(time.Duration(delay) * time.Second)
	// to one
	manager.Inc()
	testData := connectionsupport.TestingData(manager)
	counter, total := testData.GetTestingData()
	t.Logf("counter: %d total: %d", counter, total)
	if counter != int64(wCount * operCount + 1) {
		t.Error("Counter incorrect value!")		
	}
	if total != 1 {
		t.Error("Total value is incorrect!")
	}
}

func TestIdIntersection(t *testing.T) {
	option := options.SysOption{}
	manager := connectionsupport.NewConnectionDataManager(option)
	wCount := 10
	operCount := 1000
	// check in coroutine
	result := make(map[int64]rune)
	buffer := make(chan int64, wCount)
	worker := func(m *connectionsupport.ConnectionDataManager, out *chan int64) {
		for i := 0; i < operCount; i ++ {
			connData := m.NewConnection()
			newId := connData.GetId()
			*out <- newId
		}
		*out <- -1
	}
	for index := 0; index < wCount; index ++ {
		go worker(manager, &buffer)
	}
	exitFlagsCount := 0
	for exitFlagsCount < wCount {
		newId := <- buffer
		if newId > 0 {
			result[newId] = ' '
		} else {
			exitFlagsCount ++
		}			
		if exitFlagsCount >= wCount {
			t.Logf("%d worker finished", exitFlagsCount)
		}
	}
	close(buffer)
	size := len(result)
	testData := connectionsupport.TestingData(manager)
	counter, total := testData.GetTestingData()
	t.Logf("counter: %d total: %d", counter, total)
	t.Logf("Unique id count: %d", size)
	if size != wCount * operCount {
		t.Error("Problem with ID generation!")
	}
}

func TestIndexCorrect(t *testing.T) {
	option := options.SysOption{}
	manager := connectionsupport.NewConnectionDataManager(option)
	operCount := 1000
	result := make(map[int]int)
	for i := 0; i < operCount; i ++ {
		connData := manager.NewConnection()
		newIndex := connData.GetResourceIndex()
		value, exists := result[newIndex]
		if !exists {
			value = 1
		} else {
			value ++
		}
		result[newIndex] = value
	}

	size := len(result)
	testData := connectionsupport.TestingData(manager)
	counter, total := testData.GetTestingData()
	t.Logf("counter: %d total: %d", counter, total)
	t.Logf("Unique index count: %d", size)
	if size != int(operCount / connectionsupport.ResourcesGroupSize) {
		t.Error("Problem with index calculation!")
	}
}

func TestCidConvertation(t *testing.T) {
	var cid string
	cid = "afaad-ffff-2314"
	connData, err := connectionsupport.ExtractConnectionData(&cid)
	if err != nil {
		t.Error("Cid must be correct!")
	} else {
		if connData.GetId() == 65535 {
			t.Logf("connection data: %s", connData)			
		} else {
			t.Error("Convertation problem!")
		}
	}
	cid = "afaad-ff4fkf6-2314"
	connData, err = connectionsupport.ExtractConnectionData(&cid)
	if err == nil {
		t.Error("Convertation problem!")
	}
	cid = "afaad-ff4f0f6--2314"
	connData, err = connectionsupport.ExtractConnectionData(&cid)
	if err == nil {
		t.Error("Convertation problem!")
	}
}
