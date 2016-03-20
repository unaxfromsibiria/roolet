package connectionsupport

import (
	"fmt"
	"errors"
	"sync"
	"strings"
	"strconv"
	"roolet/helpers"
	"roolet/options"
	"roolet/rllogger"
)

const (
	ResourcesGroupSize = 100
	// design value
	MaxClientCount = 100000
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

func ExtractConnectionData(cid string) (*ConnectionData, error) {
	parts := strings.Split(cid, "-")
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

type ClientStateData struct {
	Busy bool
}

func newClientStateData() *ClientStateData {
	result := ClientStateData{}
	return &result
}

func (stateData *ClientStateData) clear() {
	(*stateData).Busy = false
}

type ConnectionDataStorageCell struct {
	safeObject
	data map[int64]*ClientStateData
}

func (cell *ConnectionDataStorageCell) create(id int64) {
	cell.Lock(true)
	defer cell.Unlock(true)
	(*cell).data[id] = newClientStateData()
}

func (cell *ConnectionDataStorageCell) Clear(id int64) {
	cell.Lock(true)
	defer cell.Unlock(true)
	if rec, exists := (*cell).data[id]; exists {
		rec.clear()
	}
}

func (cell *ConnectionDataStorageCell) IsBusy(id int64) bool {
	cell.Lock(false)
	defer cell.Unlock(false)
	if rec, exists := (*cell).data[id]; exists {
		return (*rec).Busy
	} else {
		return false
	}
}

func (cell *ConnectionDataStorageCell) SetBusy(id int64, value bool) {
	cell.Lock(true)
	defer cell.Unlock(true)
	if rec, exists := (*cell).data[id]; exists {		
		(*rec).Busy = value
	}
}

func newConnectionDataStorageCell() *ConnectionDataStorageCell {
	result := ConnectionDataStorageCell{
		data: make(map[int64]*ClientStateData),
		safeObject: safeObject{changeLock: new(sync.RWMutex)}}
	return &result
}

type ConnectionDataManager struct {
	safeObject
	rand helpers.SysRandom
	options options.SysOption
	storage []*ConnectionDataStorageCell
	index int64
	total int64
}

func NewConnectionDataManager(options options.SysOption) *ConnectionDataManager {
	rand := helpers.NewSystemRandom()
	result := ConnectionDataManager{
		rand: *rand,
		options: options,
		// all pointer set reserved now
		storage: make([]*ConnectionDataStorageCell, MaxClientCount / ResourcesGroupSize),
		safeObject: safeObject{changeLock: new(sync.RWMutex)}}
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
	var index int
	if total <= ResourcesGroupSize {
		index = 1
	} else {
		index = int((total - 1) / ResourcesGroupSize) + 1
	}
	if index > MaxClientCount / ResourcesGroupSize {
		index = MaxClientCount / ResourcesGroupSize
		rllogger.Outputf(rllogger.LogError, "To muth clients! Over %d!", MaxClientCount)
	}
	if (*manager).storage[index - 1] == nil {
		(*manager).storage[index - 1] = newConnectionDataStorageCell()
	}
	manager.Unlock(true)
	(*manager).storage[index - 1].create(value)
	connectionData := newConnectionData(prefix, value, index)
	return connectionData
}

func (manager *ConnectionDataManager) CheckStorageExists(index int) bool {
	manager.Lock(false)
	defer manager.Unlock(false)
	return (
		index >= 1 && index <= (MaxClientCount / ResourcesGroupSize) &&
		manager.storage[index - 1] != nil)
}

func (manager *ConnectionDataManager) ClientBusy(cid string) bool {
	result := false
	if connData, err := ExtractConnectionData(cid); err == nil {
		result = (
			manager.CheckStorageExists(connData.index) &&
			manager.storage[connData.index - 1].IsBusy(connData.id))
	}
	return result
}

func (manager *ConnectionDataManager) SetClientBusy(cid string, value bool) bool {
	result := false
	if connData, err := ExtractConnectionData(cid); err == nil {
		result = manager.CheckStorageExists(connData.index)
		if result {
			manager.storage[connData.index - 1].SetBusy(connData.id, value)			
		}
	}
	return result
}

// testing only (not use it)
type TestingData interface {
    GetTestingData() (int64, int64)
}

func (manager ConnectionDataManager) GetTestingData() (int64, int64) {
	return manager.index, manager.total
}
