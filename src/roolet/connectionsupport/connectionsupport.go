package connectionsupport

import (
	"errors"
	"fmt"
	"roolet/helpers"
	"roolet/options"
	"roolet/rllogger"
	"strconv"
	"strings"
	"sync"
)

const (
	ResourcesGroupSize = 100
	// design value
	MaxClientCount = 100000
	GroupCount     = MaxClientCount / ResourcesGroupSize
	// clients group
	GroupConnectionServer = 1
	GroupConnectionClient = 2
	GroupConnectionWsClient = 3
)

type AsyncSafeObject struct {
	changeLock *sync.RWMutex
}

func NewAsyncSafeObject() *AsyncSafeObject {
	obj := AsyncSafeObject{changeLock: new(sync.RWMutex)}
	return &obj
}

func (obj *AsyncSafeObject) Lock(rw bool) {
	if rw {
		(*obj).changeLock.Lock()
	} else {
		(*obj).changeLock.RLock()
	}
}

func (obj *AsyncSafeObject) Unlock(rw bool) {
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
	id    int64
	auth  bool
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
	Busy     bool
	TempData []byte
	auth bool
	group int
}

func newClientStateData() *ClientStateData {
	result := ClientStateData{}
	return &result
}

func (stateData *ClientStateData) clear() {
	(*stateData).Busy = false
}

// update state way
type ClientDataUpdater interface {
	update(state *ClientStateData)
}

type StateChanges struct {
	// accepted field for changes in base state
	Auth bool
	ConnectionClientGroup int
}

func (changes StateChanges) update(state *ClientStateData) {
	(*state).auth = changes.Auth
	(*state).group = changes.ConnectionClientGroup
}

// part of client data map
type ConnectionDataStorageCell struct {
	AsyncSafeObject
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

func (cell *ConnectionDataStorageCell) Remove(id int64) {
	cell.Lock(true)
	defer cell.Unlock(true)
	if _, exists := (*cell).data[id]; exists {
		delete((*cell).data, id)
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

func newConnectionDataStorageCell() *ConnectionDataStorageCell {
	objPtr := NewAsyncSafeObject()
	result := ConnectionDataStorageCell{
		data:            make(map[int64]*ClientStateData),
		AsyncSafeObject: *objPtr}
	return &result
}

type ConnectionStateChecker interface {
	ClientInGroup(cid string, group int) bool
	ClientBusy(cid string) bool
	CheckStorageExists(index int) bool
	IsAuth(cid string) bool
}

type ConnectionDataManager struct {
	AsyncSafeObject
	rand    helpers.SysRandom
	options options.SysOption
	storage []*ConnectionDataStorageCell
	index   int64
	total   int64
}

func NewConnectionDataManager(options options.SysOption) *ConnectionDataManager {
	rand := helpers.NewSystemRandom()
	result := ConnectionDataManager{
		rand:    *rand,
		options: options,
		// all pointer set reserved now
		storage:         make([]*ConnectionDataStorageCell, GroupCount),
		AsyncSafeObject: AsyncSafeObject{changeLock: new(sync.RWMutex)}}
	return &result
}

func (manager *ConnectionDataManager) Close() {
	// pass
}

func (manager *ConnectionDataManager) Inc() int64 {
	manager.Lock(true)
	defer manager.Unlock(true)
	value := (*manager).index + 1
	(*manager).index = value
	(*manager).total++
	return value
}

func (manager *ConnectionDataManager) Dec() {
	manager.Lock(true)
	defer manager.Unlock(true)
	(*manager).total--
}

func (manager *ConnectionDataManager) NewConnection() *ConnectionData {
	manager.Lock(true)
	value := (*manager).index + 1
	total := (*manager).total + 1
	(*manager).index = value
	(*manager).total = total
	prefix := manager.rand.GetShotHexPrefix()
	var index int
	if total <= ResourcesGroupSize {
		index = 1
	} else {
		index = int((total-1)/ResourcesGroupSize) + 1
	}
	if index > MaxClientCount/ResourcesGroupSize {
		index = MaxClientCount / ResourcesGroupSize
		rllogger.Outputf(rllogger.LogError, "To muth clients! Over %d!", MaxClientCount)
	}
	if (*manager).storage[index-1] == nil {
		(*manager).storage[index-1] = newConnectionDataStorageCell()
	}
	manager.Unlock(true)
	(*manager).storage[index-1].create(value)
	connectionData := newConnectionData(prefix, value, index)
	return connectionData
}

// update state data of connection by CID
func (manager *ConnectionDataManager) UpdateState(cid string, updater ClientDataUpdater) {
	if connData, err := ExtractConnectionData(cid); err == nil {
		if manager.CheckStorageExists(connData.index) {
			manager.Lock(true)
			updater.update(manager.storage[connData.index-1].data[connData.id])
			manager.Unlock(true)
		}
	}
}

func (manager *ConnectionDataManager) CheckStorageExists(index int) bool {
	manager.Lock(false)
	defer manager.Unlock(false)
	return (index >= 1 && index <= (MaxClientCount/ResourcesGroupSize) &&
		manager.storage[index-1] != nil)
}

func (manager *ConnectionDataManager) ClientBusy(cid string) bool {
	result := false
	if connData, err := ExtractConnectionData(cid); err == nil {
		result = (manager.CheckStorageExists(connData.index) &&
			manager.storage[connData.index-1].IsBusy(connData.id))
	}
	return result
}

func (manager *ConnectionDataManager) RemoveConnection(cid string) {
	if connData, err := ExtractConnectionData(cid); err == nil {
		manager.storage[connData.index-1].Remove(connData.id)
	}
}

func (manager *ConnectionDataManager) ClientInGroup(cid string, group int) bool {
	result := false
	if connData, err := ExtractConnectionData(cid); err == nil {
		cell := manager.storage[connData.index-1]
		cell.Lock(false)
		defer cell.Unlock(false)
		if rec, exists := (*cell).data[connData.id]; exists {
			result = (*rec).group == group
		}
	}
	return result
}

func (manager *ConnectionDataManager) IsAuth(cid string) bool {
	result := false
	if connData, err := ExtractConnectionData(cid); err == nil {
		cell := manager.storage[connData.index-1]
		cell.Lock(false)
		defer cell.Unlock(false)
		if rec, exists := (*cell).data[connData.id]; exists {
			result = (*rec).auth
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
