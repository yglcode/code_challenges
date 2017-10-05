/* ExpireMap: expiring cache
 * Requirements of ExpireMap:
 * 1. thread safe.
 * 2. only take space proportional to the current number of entries in it.
 * 3. timeout should be enforced as accurately as the underlying operating system allows.
 * 4. be efficient in the big O time of each of three methods in the interface.
 *
interface ExpireMapIntf<K,V> {
	//If there is no entry with the key in the map, add the key/value pair as a new entry. If there is an existing entry with the key, the current entry will be replaced with the new key/value pair. If the newly added entry is not removed after timeoutMs since it's added to the map, remove it. assuming timeout is specified in milliseconds.
	void put(K key, V value,long timeoutMs);

	//Get the value associated with the key if present; otherwise, return null.
	V get(K key);

	//Remove the entry associated with key, if any.
	void remove(K key);

	//return map size
	int size();
}
*/
package expcache

import (
	//"fmt"
	"math"
	"time"
	"sync"
	"sort"
)

//pair of data and its timeout time
type TimedData struct {
	data interface{}
	timeout int64
}

//run a purger goroutine in background, waiting for min timeout and wake
//up do purging
type Cache struct {
	//-- cache map --
	data map[interface{}]TimedData //TimedData{data:(val - interface{}),timeout)
	sync.RWMutex //lock to sync cache opers: Put/Get/Remove, also protect operLog

	//-- timeout management --
	tm *TimeoutManager
}

//create a new cache, spawning background purging goroutine
func New(args ...int) (c *Cache) {
	c = &Cache{}
	c.data = make(map[interface{}]TimedData)
	c.tm = newTimeoutManager(c, args...)
	return
}

//shutdown timeout manager
func (c *Cache) Close() {
	c.tm.Close()
}

//add an entry to cache, timeout in milliseconds
func (c *Cache) Put(k, v interface{}, timeoutMs int64) {
	if timeoutMs <= 0 {
		return
	}
	c.Lock() //lock for writing
	ent,existed := c.data[k]
	if !existed {
		ent = TimedData{}
	}
	oTo := ent.timeout
	ent.data, ent.timeout = v, time.Now().UnixNano()+timeoutMs*int64(time.Millisecond)
	c.data[k] = ent
	//append "ADD" opers to Timeout Manager's oper log
	needResched := c.tm.appendAddOper(k, oTo, ent.timeout)
	c.Unlock()
	if needResched {
		c.tm.notifyResched()
	}
}

//get entry assoc with key
func (c *Cache) Get(k interface{}) (v interface{}) {
	c.RLock()
	defer c.RUnlock()
	vt,ok := c.data[k]
	if ok {
		v = vt.data
	}
	return
}

//remove entry assoc with key
func (c *Cache) Remove(k interface{}) (v interface{}) {
	c.Lock()
	vt,ok := c.data[k]
	if !ok {
		c.Unlock()
		return
	}
	v = vt.data
	delete(c.data, k)
	//append "DEL" opers to Timeout Manager's oper log
	needResched := c.tm.appendDelOper(k, vt.timeout)
	c.Unlock()
	if needResched {
		c.tm.notifyResched()
	}
	return
}

//return number of entries in cache
func (c *Cache) Size() int {
	c.RLock()
	defer c.RUnlock()
	return len(c.data)
}

//return cache's content as map[interface{}]interface{}
func (c *Cache) GetContent() (m map[interface{}]interface{}) {
	c.RLock()
	defer c.RUnlock()
	if len(c.data) > 0 {
		m = make(map[interface{}]interface{})
		for k, vt := range c.data {
			m[k] = vt.data
		}
	}
	return
}

//--- timeout management related ---

//config parameters to make sure oper Log size limited (req: proportional to data size)
const (
	//default pending list size double the size of entries in cache
	DEF_OPER_LOG_TO_DATA_RATIO = 2
	//or use 32 as max pending list size
	DEF_OPER_LOG_MAX_SIZE = 32
)

type Oper int
const (
	ADD Oper = iota
	DEL
)

type KeyOper struct {
	key interface{}
	oper Oper
}

//memory pools to avoid garbage collection
var (
    keyOperPool sync.Pool
)

func newKeyOper(k interface{}, o Oper) *KeyOper {
     if v := keyOperPool.Get(); v != nil {
        ko := v.(*KeyOper)
        ko.key, ko.oper = k, o
        return ko
     }
     return &KeyOper{k,o}
}

func freeKeyOper(ko *KeyOper) {
     ko.key = nil
     keyOperPool.Put(ko)
}

//TimeoutManager manages cache entry timeouts and run a background goroutine for cleanup
type TimeoutManager struct {
	cache *Cache
	//-- api area shared with cache operations: Put/Get/Remove --
	//new/pending (oper,timeout)s to be processed and update timeouts list
	operLog []TimedData //TimedData{data:(keyOper - {key,ADD/DEL}),timeout}
	nextPurgeTime int64 //next time to purge, ie purger goroutine wake up
	cacheSize int //maintain a local counter to avoid locking cache to get size
	//above 3 items are shared between Cache client goroutines and TimeoutManager
	//background "purger" goroutine, protected by Cache.Lock() (WriteLock)

	//-- timeout manager internal data --
	//timeout list sorted from earliest to latest
	//entries are expired from cache according to order in this list
	timeouts []TimedData //TimedData{data:(keys - []interface{}),timeout}
	pendingRatio, pendingMax int
	//notify background purging goroutine to wake up
	resched chan struct{}
	purgeTimer *time.Timer //timer to wake up background purger to cleanup
}

func newTimeoutManager(cac *Cache, args ...int) (c *TimeoutManager) {
	c = &TimeoutManager{}
	c.cache = cac
	c.pendingRatio = DEF_OPER_LOG_TO_DATA_RATIO
	if len(args) > 0 {
		c.pendingRatio = args[0]
	}
	c.pendingMax = DEF_OPER_LOG_MAX_SIZE
	if len(args) > 1 {
		c.pendingMax = args[1]
	}
	c.resched = make(chan struct{},1) //buffered: non-blocking notif, closed when shutdown
	//create purge timer and stop it right away; later reset it when got exact timeout
	c.purgeTimer = time.NewTimer(time.Duration(math.MaxInt64))
	c.purgeTimer.Stop()
	//init nextPurgeTime so any real timeouts are before it
	c.nextPurgeTime = math.MaxInt64
	go c.purger()
	return
}

//shutdown background purger goroutine
func (c *TimeoutManager) Close() {
	c.purgeTimer.Stop()
	close(c.resched)
}

//append "ADD" opers to Timeout Manager's oper log
//called from Cache, assuming cache.Lock() hold
func (c *TimeoutManager) appendAddOper(key interface{}, oTo, nTo int64) bool {
	if oTo == 0 { //since old timeout = 0, cache add a new entry
		c.cacheSize++
	} else {
		//entry already exist; to change its timeout, first remove it from
		//timeouts list and then insert it
		c.operLog = append(c.operLog, TimedData{newKeyOper(key, DEL), oTo})
	}
	c.operLog = append(c.operLog, TimedData{newKeyOper(key, ADD), nTo})
	if oTo == c.nextPurgeTime || //if removed old entry is head of timeout list, resched
		nTo < c.nextPurgeTime || //if earlier than head timeout list, resched
		len(c.operLog) > c.pendingLimit() {
		return true //need wakeup background purger goroutine 
	}
	return false
}

//append "DEL" opers to Timeout Manager's oper log
//called from Cache, assuming cache.Lock() hold
func (c *TimeoutManager) appendDelOper(k interface{}, to int64) bool {
	c.cacheSize--
	c.operLog = append(c.operLog, TimedData{newKeyOper(k,DEL),to})
	if to == c.nextPurgeTime ||
		len(c.operLog) > c.pendingLimit() {
		return true  //need wakeup background purger goroutine 
	}
	return false
}

//called from within lock
func (c *TimeoutManager) notifyResched() {
	//non-blocking send on c.resched
	select {
	case c.resched <- struct{}{}:
	default:
	}
}

//called from within lock
func (c *TimeoutManager) pendingLimit() (pl int) {
	pl = c.pendingRatio*c.cacheSize
	if pl < c.pendingMax {
		pl = c.pendingMax
	}
	return
}

//-- the following are methods only called from background "purger" goroutine

func (c *TimeoutManager) addKeyTimeout(k interface{}, i int) {
	keys := c.timeouts[i].data.([]interface{})
	c.timeouts[i].data = append(keys, k)
	//fmt.Println("add key: ",k, c.timeouts[i].timeout)
}

func (c *TimeoutManager) delKeyTimeout(k interface{}, i int) {
	keys := c.timeouts[i].data.([]interface{})
	//99% chance, keys list has one key
	if k == keys[0] {
		//fmt.Println("del key: ",k)
		keys = keys[1:]
	} else {
		for i:=1;i<len(keys);i++ {
			if k == keys[i] {
				//remove it
				//fmt.Println("del key: ",k)
				keys = append(keys[:i], keys[i+1:]...)
				break;
			}
		}
	}
	if len(keys) == 0 {
		//no more keys at this timeout, remove it
		c.timeouts = append(c.timeouts[:i], c.timeouts[i+1:]...)
	} else {
		c.timeouts[i].data = keys
	}
}

func (c *TimeoutManager) updateTimeoutsFromPendingList() {
	//first grab all pending opers to avoid blocking cache Put/Remove() opers
	c.cache.Lock()
	operLog := c.operLog
	c.operLog = nil
	c.cache.Unlock()
	//process oper log records and apply changes to timeout list
	for indx:=0;indx<len(operLog);indx++ {
		kot := operLog[indx]
		//c.operLog[indx] = nil
		//kot: key oper & timeout
		ko := kot.data.(*KeyOper)
		//binary search find entry with ko.timeout
		i := sort.Search(len(c.timeouts), func(n int) bool {
			return c.timeouts[n].timeout >= kot.timeout
		})
		//if found timedData entry, add/del key at timedData entry
		if i<len(c.timeouts) && c.timeouts[i].timeout == kot.timeout {
			if ko.oper == DEL {
				c.delKeyTimeout(ko.key, i)
			} else if ko.oper == ADD {
				c.addKeyTimeout(ko.key, i)
			}
		} else if ko.oper == ADD { //not found, only for ADD
			l := len(c.timeouts)
			c.timeouts = append(c.timeouts,TimedData{})
			if i != l {
				//right shift one position
				copy(c.timeouts[i+1:l+1],c.timeouts[i:l])
			}
			c.timeouts[i] = TimedData{make([]interface{},0,3), kot.timeout}
			c.addKeyTimeout(ko.key, i)
		}
		freeKeyOper(ko)
	}
	operLog = operLog[:0]
}

func (c *TimeoutManager) deleteExpiredEntries(now int64) {
	//fmt.Println("deleteExpiredEntries1 ",now,c.timeouts[0].timeout)
	if len(c.timeouts)>0 && now >= c.timeouts[0].timeout { //if there are expired entries, lock cache
		c.cache.Lock()
		defer c.cache.Unlock()
		//fmt.Println("some key expires")
	} else {
		return
	}
	for len(c.timeouts)>0 {
		if now < c.timeouts[0].timeout {
			break
		} else {
			//found expired
			keys := c.timeouts[0].data.([]interface{})
			for _, k := range keys {
				delete(c.cache.data,k)
				//fmt.Println("key expire: ",k)
			}
			c.timeouts = c.timeouts[1:]
		}
	}
}

func (c *TimeoutManager) handleResched() {
	//1. work from operLog, update timeouts list
	c.updateTimeoutsFromPendingList()
	//2. delete expired entries
	//3. find new min/earliest timeout and reset timer
	c.purgeAndResetTimer()
}


func (c *TimeoutManager) purgeAndResetTimer() {
	//fmt.Println("wake up")
	//1. delete expired entries
	now := time.Now().UnixNano()
	c.deleteExpiredEntries(now)
	//2. find new min/earliest timeout and reset timer
	if len(c.timeouts) == 0 {
		c.purgeTimer.Stop()
	} else if c.nextPurgeTime != c.timeouts[0].timeout {
		//reset timer
		c.purgeTimer.Reset(time.Duration(c.timeouts[0].timeout-now))
	}
	//3. update nextPurgeTime and synchronized with cache opers
	c.cache.Lock()
	defer c.cache.Unlock()
	if len(c.timeouts) == 0 {
		c.nextPurgeTime = math.MaxInt64
	} else if c.nextPurgeTime != c.timeouts[0].timeout {
		//reset purgetime
		c.nextPurgeTime = c.timeouts[0].timeout
	}
}

//purger goroutine waiting on three events/chans:
//1. resched: possible new min timeout, recheck and resched; when closed,  cleanup & exit
//2. timeout: entry expired, delete it
func (c *TimeoutManager) purger() {
	for ok:=true;ok; {
		select {
		case _, ok = <- c.resched: //when ok = false, chan "resched" is closed, exit
			if ok {
				//do scheduling:
				//1. work from operLog, update timeouts list
				//2. delete expired entries
				//3. find new min/earliest timeout and reset timer
				c.handleResched()
			}
		case <- c.purgeTimer.C:
			c.handleResched()
		}
	}
}

