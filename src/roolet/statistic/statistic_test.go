package statistic_test

import (
    "roolet/statistic"
    "roolet/options"
    "roolet/rllogger"
    "testing"
    "time"
)

func TestStatisticSimple(t *testing.T) {
	option := options.SysOption{
		Statistic: true, StatisticCheckTime: 2}
	stat := statistic.NewStatistic(option)
	outHandler := func(now time.Time, lines *[]string) {
		for _, line := range *lines{
			t.Log(line)
		}
	}
	calcSummMethod := func(items *map[string]statistic.StatValueType) {
		var sum statistic.StatValueType
		if val, exists := (*items)["test1"]; exists {
			sum += val
		}
		if val, exists := (*items)["test2"]; exists {
			sum += val
		}
		(*items)["test3"] = sum
	}
	rllogger.SetSilent(true)
	stat.AddStatisticOutHandler(outHandler)
	stat.AddCalcHandler(calcSummMethod)
	stat.AddItem("test1", "Test value 1")
	stat.AddItem("test2", "Test value 2")
	stat.AddItem("test3", "Test value 3")
	stat.SendMsg("test2", 10)
	stat.SendMsg("test1", 5.5)
	time.Sleep(time.Duration(option.StatisticCheckTime + 1) * time.Second)
	stat.Close()
	testData := statistic.TestingData(stat)
	items := testData.GetTestingData()
	if val, exists := (*items)["test3"]; !exists || val != 15.5 {
		t.Error("Calculation problem!")
	}
}

func TestUnexpectedClosingForAsyncClients(t *testing.T) {
	// TODO: make this test
}
