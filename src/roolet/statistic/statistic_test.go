package statistic_test

import (
	"fmt"
	"os"
	"roolet/options"
	"roolet/rllogger"
	"roolet/statistic"
	"testing"
	"time"
)

func TestStatisticSimple(t *testing.T) {
	option := options.SysOption{
		Statistic: true, StatisticCheckTime: 2}
	stat := statistic.NewStatistic(option)
	outHandler := func(now time.Time, lines *[]string) {
		for _, line := range *lines {
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
	time.Sleep(time.Duration(option.StatisticCheckTime+1) * time.Second)
	stat.Close()
	testData := statistic.TestingData(stat)
	items := testData.GetTestingData()
	if val, exists := (*items)["test3"]; !exists || val != 15.5 {
		t.Error("Calculation problem!")
	}
}

func TestStatisticFileSave(t *testing.T) {
	option := options.SysOption{
		Statistic:          true,
		StatisticFile:      fmt.Sprintf("%s/roolettest.stat.log", os.TempDir()),
		StatisticCheckTime: 1}

	t.Logf("Statistic file: %s", option.StatisticFile)
	stat := statistic.NewStatistic(option)
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
	stat.AddCalcHandler(calcSummMethod)
	stat.AddItem("test1", "Test value 1")
	stat.AddItem("test2", "Test value 2")
	stat.AddItem("test3", "Test value 3")
	stat.SendMsg("test2", 10)
	stat.SendMsg("test1", 5.5)
	time.Sleep(time.Duration(option.StatisticCheckTime+1) * time.Second)
	stat.Close()
	if file, err := os.Open(option.StatisticFile); err != nil {
		t.Error(err)
	} else {
		if fi, err := file.Stat(); err != nil {
			t.Error(err)
		} else {
			fSize := fi.Size()
			t.Logf("File size: %d", fSize)
			if fSize != 81 {
				t.Error("Statistic file format changed??")
			}
		}
		file.Close()
		os.Remove(option.StatisticFile)
	}
}

func TestUnexpectedClosingForAsyncClients(t *testing.T) {
	option := options.SysOption{
		Statistic: true, StatisticCheckTime: 2}
	stat := statistic.NewStatistic(option)
	outHandler := func(now time.Time, lines *[]string) {
		for _, line := range *lines {
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
		(*items)["summ"] = sum
	}
	calcAvgMethod := func(items *map[string]statistic.StatValueType) {
		var sum statistic.StatValueType
		if val, exists := (*items)["test1"]; exists {
			sum += val
		}
		if val, exists := (*items)["test2"]; exists {
			sum += val
		}
		(*items)["avg"] = sum / 2.0
	}
	rllogger.SetSilent(true)
	stat.AddStatisticOutHandler(outHandler)
	stat.AddCalcHandler(calcSummMethod)
	stat.AddCalcHandler(calcAvgMethod)
	stat.AddItem("test1", "Test value 1")
	stat.AddItem("test2", "Test value 2")
	stat.AddItem("count", "count")
	stat.AddItem("avg", "avg")
	stat.AddItem("summ", "summ")

	worker := func(st *statistic.Statistic, n int) {
		for i := 0; i < n; i++ {
			stat.SendMsg("test2", 1)
			stat.SendMsg("test1", 0.1)
			stat.SendMsg("test2", 0.1)
			stat.SendMsg("test1", 1)
			stat.SendMsg("count", 1)
		}
	}
	for index := 0; index <= 64; index++ {
		go worker(stat, 4000)
	}
	time.Sleep(time.Duration(100) * time.Millisecond)
	stat.Close()
	testData := statistic.TestingData(stat)
	items := testData.GetTestingData()
	if val, exists := (*items)["count"]; exists && val > 10 {
		t.Logf("count = %d", int(val))
	} else {
		t.Error("some problem")
	}
}
