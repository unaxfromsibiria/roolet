package connectionsupport

import (
	"fmt"
	"errors"
	"sync"
	"strings"
	"strconv"
	"roolet/helpers"
	"roolet/options"
)

const (
	ResourcesGroupSize = 100
)

type safeObject struct {
	changeLock *sync.RWMutex
}

func (obj *safeObject) Lock(rw bool) {
	if rw {
		(*obj).changeLock.Lock()
	} else {
		(*obj).changeLock.RLock()
	}
}

func (obj *safeObject) Unlock(rw bool) {
	if rw {
		(*obj).changeLock.Unlock()
	} else {
		(*obj).changeLock.RUnlock()
	}
}

type ConnectionData struct {
	Cid string
	// [1...n]
	index int
	id int64
}

func newConnectionData(prefix string, id int64, index int) *ConnectionData {
	result := ConnectionData{
		id: id, index: index, Cid: fmt.Sprintf("%s-%016X-%d", prefix, id, index)}
	return &result
}

func (connData ConnectionData) String() string {
	return fmt.Sprintf("%s(%d, %d)", connData.Cid, connData.id, connData.index)
}

func ExtractConnectionData(cid *string) (*ConnectionData, error) {
	parts := strings.Split(*cid, "-")
	if len(parts) != 3 {
		return nil, errors.New("Client id format error")
	}
	index, err := strconv.ParseInt(parts[2], 10, 32)
	if err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(parts[1], 16, 64)
	if err != nil {
		return nil, err
	}
	return newConnectionData(parts[0], id, int(index)), nil
}

func (connData *ConnectionData) GetId() int64 {
	return connData.id
}

func (connData *ConnectionData) GetResourceIndex() int {
	return connData.index
}

type ConnectionDataStorageCell struct {
	safeObject
}

type ConnectionDataManager struct {
	safeObject
	rand helpers.SysRandom
	options options.SysOption
	index int64
	total int64
}

func NewConnectionDataManager(options options.SysOption) *ConnectionDataManager {
	rand := helpers.NewSystemRandom()
	result := ConnectionDataManager{
		rand: *rand, options: options, safeObject: safeObject{changeLock: new(sync.RWMutex)}}
	return &result
}

func (manager *ConnectionDataManager) Inc() int64 {
	manager.Lock(true)
	defer manager.Unlock(true)
	value := (*manager).index + 1
	(*manager).index = value
	(*manager).total ++
	return value
}

func (manager *ConnectionDataManager) Dec() {
	manager.Lock(true)
	defer manager.Unlock(true)
	(*manager).total --
}

func (manager *ConnectionDataManager) NewConnection() *ConnectionData {
	manager.Lock(true)
	value := (*manager).index + 1
	total := (*manager).total + 1
	(*manager).index = value
	(*manager).total = total
	prefix := manager.rand.GetShotPrefix()
	manager.Unlock(true)
	var index int
	if total <= ResourcesGroupSize {
		index = 1
	} else {
		index = int(total / ResourcesGroupSize)
	}
	connectionData := newConnectionData(prefix, value, index)
	return connectionData
}

// testing only (not use it)
type TestingData interface {
    GetTestingData() (int64, int64)
}

func (manager ConnectionDataManager) GetTestingData() (int64, int64) {
	return manager.index, manager.total
}
