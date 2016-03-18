package connectionsupport_test

import (
    "roolet/connectionsupport"
    "testing"
    "time"
)

func TestConcurrencyCounterUse(t *testing.T) {
	ac := connectionsupport.NewConnectionAccounting()
	w_count := 10
	oper_count := 1000
	worker := func(accounting *connectionsupport.ConnectionAccounting, inc bool) {
		for i := 0; i < oper_count; i ++ {
			if inc {
				accounting.Inc()				
			} else {
				accounting.Dec()
			}
		}
	}
	for index := 0; index < w_count; index ++ {
		go worker(ac, true)
		go worker(ac, false)
	}
	delay := 3.2
	t.Logf("Wait %f sec.", delay)
	time.Sleep(time.Duration(delay) * time.Second)
	// to one
	ac.Inc()
	testData := connectionsupport.TestingData(ac)
	counter, total, rev_c := testData.GetTestingData()
	t.Logf("counter: %d total: %d reverse counter: %d", counter, total, rev_c)
	if counter != w_count * oper_count + 1 {
		t.Error("Counter incorrect value!")		
	}
	if total != 1 {
		t.Error("Total value is incorrect!")
	}
	if rev_c != -1 * w_count * oper_count {
		t.Error("Reverse counter is incorrect!")
	}
}

func TestResourceIndexCalc(t *testing.T) {
	ac := connectionsupport.NewConnectionAccounting()
	size := 1000
	for index := 0; index < size; index ++ {
		ac.Inc()
	}
	var index int
	index = ac.GetResourceIndex(1000)
	t.Logf("Index = %d", index)
	if index != 10 {
		t.Error("Index calculation failed!")
	}
	size = 500
	for index := 0; index < size; index ++ {
		ac.Dec()
	}
	index = ac.GetResourceIndex(1000)
	t.Logf("Index = %d", index)
	if index != 5 {
		t.Error("Index calculation failed!")
	}
	size = 500
	for index := 0; index < size; index ++ {
		ac.Inc()
	}
	index = ac.GetResourceIndex(1000)
	t.Logf("Index = %d", index)
	if index != 6 {
		t.Error("Index calculation failed!")
	}
	index = ac.GetResourceIndex(1450)
	t.Logf("Index = %d", index)
	if index != 9 {
		t.Error("Index calculation failed!")
	}
}
