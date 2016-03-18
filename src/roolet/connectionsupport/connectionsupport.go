package connectionsupport

import (
	"sync"
)

const (
	ResourcesGroupSize = 100
)

type NewConnectionIndex struct {
	index int
	group int
}

type ConnectionAccounting struct {
	total int
	counter int
	revese_counter int
	changeLock *sync.RWMutex
}

func (accounting *ConnectionAccounting) lock(rw bool) {
	if rw {
		(*accounting).changeLock.Lock()
	} else {
		(*accounting).changeLock.RLock()
	}
}

func (accounting *ConnectionAccounting) unlock(rw bool) {
	if rw {
		(*accounting).changeLock.Unlock()
	} else {
		(*accounting).changeLock.RUnlock()
	}
}

func NewConnectionAccounting() *ConnectionAccounting {
	result := ConnectionAccounting{changeLock: new(sync.RWMutex)}
	return &result
}

func (accounting *ConnectionAccounting) Inc() (int, int) {
	accounting.lock(true)
	defer accounting.unlock(true)
	value := (*accounting).counter + 1
	(*accounting).counter = value
	(*accounting).total ++
	return value, accounting.getResourceIndex(value)
}

func (accounting *ConnectionAccounting) Dec() {
	accounting.lock(true)
	defer accounting.unlock(true)
	(*accounting).total --
	(*accounting).revese_counter --
}

func (accounting *ConnectionAccounting) GetNewIndex() *NewConnectionIndex {
	result := NewConnectionIndex{}
	result.index, result.group = accounting.Inc()
	return &result
}

// index for access to allocated (multi mutex map etc.) resources by connection index
func (accounting *ConnectionAccounting) GetResourceIndex(index int) int {
	accounting.lock(false)
	defer accounting.unlock(false)
	return accounting.getResourceIndex(index)
}

func (accounting *ConnectionAccounting) getResourceIndex(index int) int {
	if accounting.total > ResourcesGroupSize && index > ResourcesGroupSize {
		groupCount := int(accounting.total / ResourcesGroupSize)
		stepSize := int(accounting.counter / groupCount)
		return int(index / stepSize)
	} else {
		return 0
	}
}

// testing only (not use it)
type TestingData interface {
    GetTestingData() (int, int, int)
}

func (accounting ConnectionAccounting) GetTestingData() (int, int, int) {
	return accounting.counter, accounting.total, accounting.revese_counter
}
