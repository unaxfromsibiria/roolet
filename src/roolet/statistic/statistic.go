package statistic

import (
	"bufio"
	"fmt"
	"os"
	"roolet/helpers"
	"roolet/options"
	"roolet/rllogger"
	"time"
)

type StatValueType float32

func (num StatValueType) String() string {
	if num - StatValueType(int(num)) <= 0 {
		return fmt.Sprintf("%d", int(num))
	} else {
		return fmt.Sprintf("%f", num)
	}
}

const (
	StatBufferSize = 512
	HandlerCountLimit = 100
	DefaultIterTime = 5
)

type StatMsg struct {
	code string
	value StatValueType
}

type AutoCalcHandler func(*map[string]StatValueType)
type StatisticOutHandler func(time.Time, *[]string)

func NewStatMsg(code string, value interface{}) *StatMsg {
	var statValue StatValueType
    switch val := value.(type) {
        case int: statValue = StatValueType(val)
        case int32: statValue = StatValueType(val)
        case int64: statValue = StatValueType(val)
        case uint32: statValue = StatValueType(val)
        case uint64: statValue = StatValueType(val)
        case uintptr: statValue = StatValueType(val)
        case float32: statValue = StatValueType(val)
        case float64: statValue = StatValueType(val)
        default: rllogger.Outputf(
            rllogger.LogTerminate,
            "Statistic not accept this type: %T", value)
    }
    msg := StatMsg{code: code, value: statValue}
    return &msg
}

type Statistic struct {
	closeChan chan bool
	messages chan StatMsg
	calcHandlers []AutoCalcHandler
	outHandlers []StatisticOutHandler
	items map[string]StatValueType
	labels map[string]string
	silent bool
	active bool
	iterTime int
	msgCount int64
	lastSendTime time.Time
}

func statProcessing(stat *Statistic) {
	timer := time.NewTicker(time.Duration((*stat).iterTime) * time.Second)
	out := func() {
		rllogger.Output(rllogger.LogInfo, "Statistic closing...")
		timer.Stop()
		close((*stat).closeChan)
		close((*stat).messages)
	}
	defer out()
	for (*stat).active {
		select {
			case msg := <-(*stat).messages: {
				stat.add(&msg)
			}
			case <- (*stat).closeChan: {
				stat.stop()
			}
			case now := <- timer.C: {
				(*stat).lastSendTime = now
				stat.calc()
				stat.SendResult()
			}
		}
	}
}

func NewStatistic(option options.SysOption) *Statistic {
	stat := Statistic{
		active: true,
		messages: make(chan StatMsg, StatBufferSize),
		closeChan: make(chan bool, 1), 
		iterTime: option.StatisticCheckTime,
		calcHandlers: make([]AutoCalcHandler, HandlerCountLimit),
		items: make(map[string]StatValueType),
		labels: make(map[string]string),
		silent: !option.Statistic}
	
	if stat.iterTime < 1 {
		stat.iterTime = DefaultIterTime
	}
	
	go statProcessing(&stat)
	// save to file support
	if len(option.StatisticFile) > 0 {
		statFilePath := option.StatisticFile
		logFileSaveHandler := func(now time.Time, lines *[]string) {
			// copy file
			newFilePath := fmt.Sprintf("%s.%d.tmp", statFilePath, now.Unix())
			hasCopy := helpers.CopyFile(newFilePath, statFilePath) == nil
			if hasCopy {
				os.Remove(statFilePath)
			}
			if newFile, err := os.Create(statFilePath); err == nil {
				defer newFile.Close()
				writer := bufio.NewWriter(newFile)
				for _, line := range *lines {
					writer.WriteString(line)
					writer.WriteRune('\n')
					writer.Flush()
				}
			} else {
				rllogger.Outputf(rllogger.LogWarn, "Statistic file %s not avalible!", statFilePath)
			}
			if hasCopy {
				os.Remove(newFilePath)
			}	
		}
		stat.AddStatisticOutHandler(logFileSaveHandler)
	}
	return &stat
}

func (stat *Statistic) SendMsg(code string, value interface{}) {
	// TODO: need lock for active change
	if (*stat).silent || !(*stat).active {
		return
	}
	msg := NewStatMsg(code , value)
	(*stat).messages <- (*msg)
}

func (stat *Statistic) Close() {
	if (*stat).active {
		(*stat).closeChan <- true
	}
}

func (stat *Statistic) SendResult() {
	if (*stat).silent {
		return
	}
	var lines []string
	lines = append(lines, fmt.Sprintf("message count: %d", (*stat).msgCount))
	var label string
	for code, value := range (*stat).items {
		if itemLabel, exists := (*stat).labels[code]; exists {
			label = itemLabel
		} else {
			label = code
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, value))
	}
	rllogger.OutputLines(rllogger.LogInfo, "statistic", &lines)
	if len((*stat).outHandlers) > 0 {
		for _, outHandler := range (*stat).outHandlers {
			outHandler((*stat).lastSendTime, &lines)
		}
	}
}

func (stat *Statistic) AddItem(code string, label string) {
	if _, exists := (*stat).items[code]; !exists {
		(*stat).items[code] = 0
	}
	(*stat).labels[code] = label
}

func (stat *Statistic) AddCalcHandler(handler AutoCalcHandler) {
	for index := 0; index < HandlerCountLimit; index ++ {
		if (*stat).calcHandlers[index] == nil {
			(*stat).calcHandlers[index] = handler
			break
		}
	}
}

func (stat *Statistic) AddStatisticOutHandler(handler StatisticOutHandler) {
	(*stat).outHandlers = append((*stat).outHandlers, handler)
}

func (stat *Statistic) add(msg *StatMsg) {
	if (*stat).silent {
		return
	}
	(*stat).msgCount ++
	m := *msg
	var newValue StatValueType
	if value, exists := (*stat).items[msg.code]; exists {
		newValue = value + m.value
	} else {
		newValue = m.value
	}
	(*stat).items[m.code] = newValue
}

func (stat *Statistic) calc() {
	for index := 0; index < HandlerCountLimit; index ++ {
		if (*stat).calcHandlers[index] == nil {
			break
		} else {
			(*stat).calcHandlers[index](&((*stat).items))
		}
	}
}

func (stat *Statistic) stop() {
	(*stat).active = false
}

// testing only (not use it)
type TestingData interface {
    GetTestingData() *map[string]float32
}

func (stat Statistic) GetTestingData() *map[string]float32 {
	items := make(map[string]float32)
	for code, value := range stat.items {
		items[code] = float32(value)
	}
	return &items
}
