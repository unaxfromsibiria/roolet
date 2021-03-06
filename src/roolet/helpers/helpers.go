package helpers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	acceptCahrs            = "abcdefghijkmnpqrstuvwxyz9876543210"
	acceptHexCahrs         = "abcdef9876543210"
	DefaultPasswordSize    = 64
	randPartSize           = 8
	asyncDictCountParts    = 64
	asyncDictCountPartsLim = asyncDictCountParts - 1
)

func GetFullFilePath(dir, fileName string) string {
	var pwdDir string
	if !path.IsAbs(dir) {
		if workDir, err := os.Getwd(); err == nil {
			pwdDir = workDir
		}
	}
	return path.Join(pwdDir, dir, fileName)
}

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

// deprecated
type CoroutineActiveResource struct {
	active     bool
	changeLock *sync.RWMutex
}

func NewCoroutineActiveResource() *CoroutineActiveResource {
	obj := CoroutineActiveResource{active: true, changeLock: new(sync.RWMutex)}
	return &obj
}

func (obj *CoroutineActiveResource) IsActive() bool {
	(*obj).changeLock.RLock()
	defer (*obj).changeLock.RUnlock()
	return (*obj).active
}

func (obj *CoroutineActiveResource) SetActive(value bool) {
	(*obj).changeLock.Lock()
	defer (*obj).changeLock.Unlock()
	(*obj).active = value
}

func (obj *CoroutineActiveResource) LockChange() {
	(*obj).changeLock.Lock()
}

func (obj *CoroutineActiveResource) UnLockChange() {
	(*obj).changeLock.Unlock()
}

func (obj *CoroutineActiveResource) RLockChange() {
	(*obj).changeLock.RLock()
}

func (obj *CoroutineActiveResource) RUnLockChange() {
	(*obj).changeLock.RUnlock()
}

type SysRandom struct {
	rand.Rand
	passwordChars string
}

func (sysRand *SysRandom) FromRangeInt(min int, max int) int {
	return min + sysRand.Intn(max-min)
}

func (sysRand *SysRandom) CreateCid(size int, prefix string) string {
	buf := make([]byte, size)
	for i := 0; i < size; i++ {
		buf[i] = acceptCahrs[sysRand.Intn(len(acceptCahrs))]
	}
	return fmt.Sprintf("%s-%x", prefix, sha256.Sum224(buf))
}

func (sysRand *SysRandom) CreateTaskId() string {
	src := fmt.Sprintf("%s-%d", sysRand.GetShotPrefix(), time.Now().UTC().UnixNano())
	return fmt.Sprintf("%x", sha256.Sum224([]byte(src)))
}

func (sysRand *SysRandom) getPrefix(base string) string {
	buf := make([]byte, randPartSize)
	n := int(len(base))
	var index int
	for i := 0; i < randPartSize; i++ {
		index = sysRand.Intn(n)
		buf[i] = base[index]
	}
	return fmt.Sprintf("%s", buf)
}

func (sysRand *SysRandom) GetShotPrefix() string {
	return sysRand.getPrefix(acceptCahrs)
}

func (sysRand *SysRandom) GetShotHexPrefix() string {
	return sysRand.getPrefix(acceptHexCahrs)
}

func (sysRand *SysRandom) Select(src *[]string) string {
	index := sysRand.FromRangeInt(0, len(*src))
	return (*src)[index]
}

func (sysRand *SysRandom) CreatePassword(size int) string {
	newSize := DefaultPasswordSize
	if size > 0 {
		newSize = size
	}

	buf := make([]byte, newSize)
	for i := 0; i < newSize; i++ {
		buf[i] = acceptCahrs[sysRand.Intn(len(acceptCahrs))]
	}
	return string(buf)
}

func NewSystemRandom() *SysRandom {
	newRand := SysRandom{
		*(rand.New(rand.NewSource(time.Now().UTC().UnixNano()))),
		fmt.Sprintf("%s_%s-+=_$#@!*&", acceptCahrs, strings.ToUpper(acceptCahrs))}
	return &newRand
}

type ConnectionContext struct {
	cid     string
	auth    bool
	tmpData string
	group   int
}

func (context ConnectionContext) String() string {
	return fmt.Sprintf(
		"context(cid=%s, auth=%t, tmp=%s, group=%d)",
		context.cid, context.auth, context.tmpData, context.group)
}

type ConnectionContextUpdater interface {
	Update(context *ConnectionContext)
}

func (context *ConnectionContext) SetupTmpData(value string) {
	(*context).tmpData = value
}

func (context *ConnectionContext) DoAuth() {
	(*context).auth = true
}

func (context *ConnectionContext) GetTmpData() string {
	return (*context).tmpData
}

func (context *ConnectionContext) Clear() {
	(*context).tmpData = ""
}

func (context *ConnectionContext) IsAuth() bool {
	return (*context).auth
}

func (context *ConnectionContext) SetCid(value string) {
	// set it only once
	if len((*context).cid) == 0 {
		(*context).cid = value
	}
}

func (context *ConnectionContext) GetCid() (string, bool) {
	exists := len(context.cid) > 0
	return context.cid, exists
}

func (context *ConnectionContext) SetClientGroup(value int) {
	// set it only once
	if (*context).group == 0 {
		(*context).group = value
	}
}

func (context *ConnectionContext) GetClientGroup() int {
	return (*context).group
}

// -----
type ServerBusyAccounting struct {
	CoroutineActiveResource
	states map[string]bool
}

func NewServerBusyAccounting() *ServerBusyAccounting {
	accounting := ServerBusyAccounting{
		*(NewCoroutineActiveResource()),
		make(map[string]bool)}
	return &accounting
}

func (accounting *ServerBusyAccounting) SetBusy(cid string, value bool) {
	accounting.LockChange()
	defer accounting.UnLockChange()
	(*accounting).states[cid] = value
}

func (accounting *ServerBusyAccounting) IsBusy(cid string) bool {
	accounting.RLockChange()
	defer accounting.RUnLockChange()
	result := false
	if busy, exists := (*accounting).states[cid]; exists {
		result = busy
	}
	return result
}

type rpcMethodsInfo struct {
	Methods []string
}

type CidList []string

func NewCidList(cid string) *CidList {
	var list CidList
	list = append(list, cid)
	return &list
}

type ServerMethods struct {
	CoroutineActiveResource
	methods map[string]CidList
}

func NewServerMethods() *ServerMethods {
	serverMethods := ServerMethods{
		*(NewCoroutineActiveResource()),
		make(map[string]CidList)}
	return &serverMethods
}

func (serverMethods *ServerMethods) Append(method string, cid string) {
	serverMethods.LockChange()
	defer serverMethods.UnLockChange()
	if _, exists := (*serverMethods).methods[method]; exists {
		(*serverMethods).methods[method] = append((*serverMethods).methods[method], cid)
	} else {
		(*serverMethods).methods[method] = *(NewCidList(cid))
	}
}

func (serverMethods *ServerMethods) IsPublic(method string) bool {
	serverMethods.RLockChange()
	defer serverMethods.RUnLockChange()
	result := false
	if _, exists := (*serverMethods).methods[method]; exists {
		result = exists
	}
	return result
}

func (serverMethods *ServerMethods) SearchFree(method string, busyAccounting *ServerBusyAccounting) (string, bool) {
	serverMethods.RLockChange()
	defer serverMethods.RUnLockChange()
	var cid string
	hasResult := false
	if cidList, exists := (*serverMethods).methods[method]; exists {
		for _, cidVariant := range cidList {
			if !busyAccounting.IsBusy(cidVariant) {
				cid = cidVariant
				hasResult = true
				break
			}
		}
	}
	return cid, hasResult
}

// content from CoreMsg has JSON format {"methods": ["method1"]}
func (serverMethods *ServerMethods) FillFromMsgData(cid string, content *[]byte) error {
	var err error
	info := rpcMethodsInfo{}
	if loadErr := json.Unmarshal(*content, &info); loadErr != nil {
		err = loadErr
	} else {
		for _, method := range info.Methods {
			serverMethods.Append(method, cid)
		}
	}
	return err
}

func CopyFile(dstFilePath, srcFilePath string) error {
	srcFile, err := os.Open(srcFilePath)
	defer srcFile.Close()
	if err != nil {
		return err
	}
	dstFile, err := os.Create(dstFilePath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		return err
	} else {
		return dstFile.Close()
	}
}

type TaskIdGenerator struct {
	index uint64
}

func (generator *TaskIdGenerator) add() {
	atomic.AddUint64(&(generator.index), 1)
}

func NewTaskIdGenerator() *TaskIdGenerator {
	result := TaskIdGenerator{}
	return &result
}

func (generator *TaskIdGenerator) CreateTaskId() string {
	generator.add()
	buf := make([]byte, randPartSize)
	for i := 0; i < randPartSize; i++ {
		buf[i] = acceptHexCahrs[rand.Intn(16)]
	}
	index := atomic.LoadUint64(&(generator.index))
	return fmt.Sprintf("%s-%016X", buf, index)
}

type asyncStrDictPart struct {
	AsyncSafeObject
	content map[string]string
}

func newasyncStrDictPart() *asyncStrDictPart {
	return &(asyncStrDictPart{
		content:         make(map[string]string),
		AsyncSafeObject: *(NewAsyncSafeObject())})
}

func (part *asyncStrDictPart) set(key, value string) {
	part.Lock(true)
	defer part.Unlock(true)
	(*part).content[key] = value
}

func (part *asyncStrDictPart) get(key string) *string {
	part.Lock(false)
	defer part.Unlock(false)
	var result *string
	if item, exists := (*part).content[key]; exists {
		result = &item
	}
	return result
}

func (part *asyncStrDictPart) exist(key string) bool {
	part.Lock(false)
	defer part.Unlock(false)
	_, result := (*part).content[key]
	return result
}

func (part *asyncStrDictPart) deleteItem(key string) bool {
	part.Lock(true)
	defer part.Unlock(true)
	if _, exists := (*part).content[key]; exists {
		delete((*part).content, key)
		return true
	}
	return false
}

func (part *asyncStrDictPart) clear() {
	part.Lock(true)
	defer part.Unlock(true)
	for key, _ := range (*part).content {
		delete((*part).content, key)
	}
}

func (part *asyncStrDictPart) size() int {
	part.Lock(false)
	defer part.Unlock(false)
	return int(len((*part).content))
}

type AsyncStrDict struct {
	pool []*asyncStrDictPart
	seek int32
}

func NewAsyncStrDict() *AsyncStrDict {
	result := AsyncStrDict{
		pool: make([]*asyncStrDictPart, asyncDictCountParts)}
	for i := 0; i < asyncDictCountParts; i++ {
		result.pool[i] = newasyncStrDictPart()
	}
	return &result
}

func (dict *AsyncStrDict) getSeek() int {
	seek := int(atomic.LoadInt32(&(dict.seek)))
	if seek >= asyncDictCountPartsLim {
		seek = asyncDictCountPartsLim
		atomic.StoreInt32(&(dict.seek), 0)
	} else {
		atomic.AddInt32(&(dict.seek), 1)
	}
	return seek
}

func (dict *AsyncStrDict) Set(key, value string) {
	seek := -1
	for i := asyncDictCountPartsLim; i >= 0; i-- {
		if dict.pool[i].exist(key) {
			// not use break here
			// this is solution for problem - very fast start Set duplicated keys
			if seek >= 0 {
				dict.pool[seek].deleteItem(key)
			}
			seek = i
		}
	}
	if seek < 0 {
		seek = dict.getSeek()
	}
	dict.pool[seek].set(key, value)
}

// fast method, only if you watch and delete keys
func (dict *AsyncStrDict) SetUnsafe(key, value string) {
	dict.pool[dict.getSeek()].set(key, value)
}

func (dict *AsyncStrDict) Get(key string) *string {
	var result *string
	mid := asyncDictCountParts / 2
	for i := 0; i < mid; i++ {
		result = dict.pool[mid+i].get(key)
		if result != nil {
			return result
		} else {
			result = dict.pool[i].get(key)
			if result != nil {
				return result
			}
		}
	}
	return result
}

func (dict *AsyncStrDict) Exists(key string) bool {
	mid := asyncDictCountParts / 2
	for i := 0; i < mid; i++ {
		if dict.pool[mid+i].exist(key) || dict.pool[i].exist(key) {
			return true
		}
	}
	return false
}

func (dict *AsyncStrDict) Delete(key string) {
	mid := asyncDictCountParts / 2
	for i := 0; i < mid; i++ {
		if dict.pool[mid+i].deleteItem(key) || dict.pool[i].deleteItem(key) {
			return
		}
	}
}

func (dict *AsyncStrDict) Clear() {
	for i := 0; i < asyncDictCountParts; i++ {
		dict.pool[i].clear()
	}
}

func (dict *AsyncStrDict) Size() int {
	var result int
	for i := 0; i < asyncDictCountParts; i++ {
		result += dict.pool[i].size()
	}
	return result
}
