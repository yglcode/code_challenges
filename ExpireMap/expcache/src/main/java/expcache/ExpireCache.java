package expcache;

import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.concurrent.locks.Condition;
import java.util.concurrent.locks.Lock;
import java.util.concurrent.locks.ReentrantLock;
import java.util.concurrent.locks.ReentrantReadWriteLock;

/*
 * Requirements of ExpireMap:
 * 1. thread safe.
 * 2. only take space proportional to the current number of entries in it.
 * 3. timeout should be enforced as accurately as the underlying operating system allows.
 * 4. be efficient in the big O time of each of three methods in the interface.
 */
interface ExpireCacheIntf<K, V> {
    // If there is no entry with the key in the map, add the key/value pair as a
    // new entry
    // If there is an existing entry with the key, the current entry will be
    // replaced with the new key/value pair.
    // If the newly added entry is not removed after timeoutMs since it's added
    // to the map, remove it.
    // assuming timeout is specified in milliseconds.
    void put(K key, V value, long timeoutMs);

    // Get the value associated with the key if present; otherwise, return null.
    V get(K key);

    // Remove the entry associated with key, if any.
    void remove(K key);

    // return map size
    int size();
}

// helper class to store a pair value
class ValueTime<S, T extends Comparable<T>> implements
                                                Comparable<ValueTime<S, T>> {
    public S value;
    public T time;

    public ValueTime(S s, T t) {
        value = s;
        time = t;
    }

    @Override
	public int compareTo(ValueTime<S, T> o) {
        return time.compareTo(o.time);
    }
}

/*
 * ExpireMap implementation notes: 1. Use a ConcurrentHashMap to store entries.
 * 2. use TimeoutManager with background worker thread to manage entry timeouts.
 * 3. off-load time-consuming tasks to worker thread: insert new timeout, find
 * timeout O(log) 4. all 3 API methods only involve constant time operations
 * O(1)
 */
public class ExpireCache<K, V> implements ExpireCacheIntf<K, V> {
    // data stores inserted (key,val) pair and its timeout.
    private final HashMap<K, ValueTime<V, Long>> data = new HashMap<K, ValueTime<V, Long>>();
    private final ReentrantReadWriteLock dataRWLock = new ReentrantReadWriteLock();
    private final Lock dataReadLock = dataRWLock.readLock();
    private final Lock dataWriteLock = dataRWLock.writeLock();
    private final TimeoutManager<K, V> tm;

    public ExpireCache() {
        tm = new TimeoutManager<K, V>(data, dataWriteLock);
    }

    public ExpireCache(long maxIdleMilliSec) {
        tm = new TimeoutManager<K, V>(maxIdleMilliSec, data, dataWriteLock);
    }

    @Override
	protected void finalize() throws Throwable {
        super.finalize();
        shutdown();
    }

    // shutdown worker thread if it still there
    public void shutdown() {
        tm.shutdown();
    }

    // assuming timeout is specified in millisecond
    @Override
	public void put(K key, V value, long timeoutMs) {
        if (timeoutMs <= 0) {
            return;
        }
        long otime = 0;
        long newTO = System.nanoTime() + timeoutMs * (long) 1e6;
        boolean needResched = false;
        dataWriteLock.lock();
        try {
            ValueTime<V, Long> vt = data.get(key);
            if (vt != null) {
                // cache del-timeout events for worker thread to process
                otime = vt.time;
                vt.value = value;
                vt.time = newTO;
            } else {
                vt = new ValueTime<V, Long>(value, newTO);
                data.put(key, vt);
            }
            // add new entry
            needResched = tm.appendAddOper(key, otime, newTO);
        } finally {
            dataWriteLock.unlock();
        }
        if (needResched) {
            tm.notifyReschedule();
        }
    }

    @Override
	public V get(K key) {
        dataReadLock.lock();
        try {
            ValueTime<V, Long> vt = data.get(key);
            if (vt == null) {
                return null;
            }
            return vt.value;
        } finally {
            dataReadLock.unlock();
        }
    }

    @Override
	public void remove(K key) {
        boolean needResched = false;
        dataWriteLock.lock();
        try {
            ValueTime<V, Long> vt = data.remove(key);
            if (vt == null) {
                return;
            }
            needResched = tm.appendDelOper(key, vt.time);
        } finally {
            dataWriteLock.unlock();
        }
        if (needResched) {
            tm.notifyReschedule();
        }
    }

    @Override
	public int size() {
        dataReadLock.lock();
        try {
            return data.size();
        } finally {
            dataReadLock.unlock();
        }
    }

}

/*
 * TimeoutManager implementation notes: 1. Use a dynamic array (ArrayList) to
 * store sorted timeouts, backed up by timeout cache: timeoutToAddDel processed
 * by worker thread 2. Use a worker thread to update timeouts list and purge
 * timeouted entries. 3. off-load time-consuming tasks to worker thread: insert
 * new timeout, find timeout O(log) 4. only wake up worker thread when has new
 * shortest timeout. 5. Use nano seconds for better precision. 6. Use
 * ReentrantLock/Condition to park worker thread and wait for timeout. 8. make
 * worker daemon thread and add Runtime.shutdownHook to clean it.
 */
class TimeoutManager<K, V> implements Runnable {
    private HashMap<K, ValueTime<V, Long>> data = null;
    private final Lock dataLock;

    // --- synced intf between Cache and background worker ---

    // nextWakeupTime and timeoutsToAddDel are synced data structure
    private volatile long nextWakeupTime;
    private final long maxSleepTime;
    // cached timeouts to be added/deled to timeouts list by worker thread
    // put,remove will only signal/wake worker thread when new timeout is closer
    // or too much timeouts pending.
    // kept in incoming order
    // encoding update actions to timeouts list:
    // ValueTime.value: 0:Add, -1:Del,>0:old-timeout to remove
    // ValueTime.time: timeout
    private final static long ADD = 0;
    private final static long DEL = -1;
    // protected by dataLock
    private HashMap<K, ValueTime<Long, Long>> timeoutsToAddDel = new HashMap<K, ValueTime<Long, Long>>();

    // --- internal implementations, not shared with Cache ---

    // configurations:
    // how long to keep worker thread when no activity;
    // after this, worker thread will exit until ExpireMap is active again.
    private final static long DEFAULT_MAX_IDLE_TIME = 3000; // default to 3
    // seconds
    // timeouts - the list of (entry-key,timeout) pair sorted by timeout.
    // keep it sorted,inserted by binary search; so it is O(log) operations;
    // not O(1) as hashmap; so let worker thread maintain it from ToAddDel list.
    // Use ArrayList to store keys with same timeout could be overkill/overhead,
    // but in case some machine don't provide precise enough timing, entries
    // could
    // end up with same timestamp.
    // only used from purger thread, no need locking
    private final ArrayList<ValueTime<ArrayList<K>, Long>> timeouts = new ArrayList<ValueTime<ArrayList<K>, Long>>();

    // Lock only used for waking up worker thread
    private final ReentrantLock wakeupLock = new ReentrantLock();
    // cond var to allow worker thread wait
    private final Condition wakeupCond = wakeupLock.newCondition();
    // the following two variables are synced by above lock between purger
    // thread
    // do i have a worker thread
    private Thread worker;
    // do i have a new min timeout, one of states when waking up worker thread
    private boolean hasNewMin;

    // shutdown hook for clean up worker thread when JVM exit
    private final Thread shutdownHook = new Thread(new Runnable() {
            @Override
            public void run() {
                shutdown();
            }
	});

    // -- the following methods are API exposed to Cache, called from other
    // client threads ---

    public TimeoutManager(HashMap<K, ValueTime<V, Long>> d, Lock dl) {
        this(DEFAULT_MAX_IDLE_TIME, d, dl);
    }

    public TimeoutManager(long maxIdleMilliSec,
                          HashMap<K, ValueTime<V, Long>> d, Lock dl) {
        data = d;
        dataLock = dl;
        maxSleepTime = maxIdleMilliSec * (long) 1e6;
        nextWakeupTime = Long.MAX_VALUE;
    }

    @Override
	protected void finalize() throws Throwable {
        super.finalize();
        shutdown();
    }

    // shutdown worker thread if it still there
    public void shutdown() {
        // System.out.println("shutdown called");
        try {
            wakeupLock.lock();
            if (worker != null) {
                worker = null; // tell active worker thread exit
                wakeupCond.signalAll();
            }
        } finally {
            wakeupLock.unlock();
        }
    }

    // called from Cache, assuming holding dataWriteLock
    public boolean appendAddOper(K key, long otime, long ntime) {
        // System.out.println("add key "+key+", otime "+otime+", ntime "+ntime);
        // check if k already exists inside map, clean up
        ValueTime<Long, Long> vt = timeoutsToAddDel.get(key);
        if (vt != null) {
            if (vt.value < 0) {// change pending DEL to DEL-ADD
                vt.value = vt.time;
            }
            vt.time = ntime;
        } else {
            // cache del-timeout events for worker thread to process
            timeoutsToAddDel.put(key, new ValueTime<Long, Long>(otime, ntime));
        }
        // System.out.println("key "+key+", nextWakeupTime "+nextWakeupTime);
        // if this entry has shorter timeout than worker currently wait on,
        // wake up worker thread and let it recheck.
        if (nextWakeupTime == Long.MAX_VALUE // worker waiting in idle
            || otime == nextWakeupTime // if this entry is the
            // shortest worker
            // currently wait on,
            // wake up worker thread and let it recheck.
            || ntime < nextWakeupTime // entry has shorter timeout
            // than worker currently
            // wait on
            ) {
            // System.out.println("find new min, wakeup purger, key "+key);
            return true; // need wakeup worker and reschedule
        }
        return false;
    }

    // called from Cache, assuming holding dataWriteLock
    public boolean appendDelOper(K key, long timeout) {
        ValueTime<Long, Long> vt = timeoutsToAddDel.get(key);
        if (vt != null) {
            if (vt.value == ADD) { // if "Add" has not applied to timeouts,
                // clear it & return
                timeoutsToAddDel.remove(key);
                return false;
            }
            if (vt.value > 0) {// change pending DEL-ADD to pure DEL
                vt.time = vt.value;
                vt.value = DEL;
            } else { // there is a pending "DEL", update it
                vt.time = timeout;
            }
        } else {
            // cache del-timeout events for worker thread to process
            timeoutsToAddDel.put(key, new ValueTime<Long, Long>(DEL, timeout));
        }

        // if worker currently waiting on this key/timeout, wakeup it
        if (timeout == nextWakeupTime) {
            return true; // need wakeup worker and reschedule
        }
        return false;
    }

    public void notifyReschedule() {
        try {
            // need wakeup worker
            wakeupLock.lock();
            if (worker == null) {
                Runtime.getRuntime().addShutdownHook(shutdownHook);
                worker = new Thread(this);
                worker.setDaemon(true); // so jvm can exit without waiting
                // for
                // worker
                worker.start();
            }
            // check if worker running, otherwise starts it
            hasNewMin = true;
            wakeupCond.signalAll();
        } finally {
            wakeupLock.unlock();
        }
    }

    // -- the following are TimeoutManager internal methods, called from
    // worker/purger thread --

    // binary search timeout entry
    // always called from worker thread
    private int findTimeout(long kTime) {
        return Collections.binarySearch(timeouts,
                                        new ValueTime<ArrayList<K>, Long>(null, kTime));
    }

    // insert timeout by binary search
    // always called from worker thread
    private void insertTimeout(K k, long kTime) {
        ValueTime<ArrayList<K>, Long> klTime = new ValueTime<ArrayList<K>, Long>(
                                                                                 null, kTime);
        int pos = Collections.binarySearch(timeouts, klTime);
        if (pos < 0) {
            klTime.value = new ArrayList<K>();
            klTime.value.add(k);
            timeouts.add(-1 - pos, klTime);
            // System.out.println("add1 "+k);
        } else {
            // this case could happen if machine don't provide enough time
            // precision.
            // some entries could end up with same timeout value.
            ValueTime<ArrayList<K>, Long> klt = timeouts.get(pos);
            klt.value.add(k);
            // System.out.println("add2 "+k);
        }
    }

    // adding pending timeout opers to timeouts list
    // get new min timeout for worker to wait
    // always called from worker thread
    private long updateAndGetMinTimeout() {
        // grab all pending timeouts to avoid conflict with client threads
        HashMap<K, ValueTime<Long, Long>> toToAddDel = null;
        dataLock.lock();
        try {
            if (timeoutsToAddDel.size() > 0) {
                toToAddDel = timeoutsToAddDel;
                timeoutsToAddDel = new HashMap<K, ValueTime<Long, Long>>();
            }
        } finally {
            dataLock.unlock();
        }
        // apply cached timeout operations
        if (toToAddDel != null)
            for (K k : toToAddDel.keySet()) {
                // apply timeout changes according to its timing order.
                ValueTime<Long, Long> op = toToAddDel.get(k);
                if (op.value == ADD) { // add a timeout
                    insertTimeout(k, op.time);
                } else if (op.value == DEL) { // del timeout
                    int pos = 0;
                    if (op.time != timeouts.get(0).time) {
                        pos = findTimeout(op.time);
                    }
                    ValueTime<ArrayList<K>, Long> klt = timeouts.get(pos);
                    klt.value.remove(k);
                    if (klt.value.size() == 0) {
                        timeouts.remove(pos);
                    }
                } else if (op.value > 0) { // del-add case
                    // op.value: old timeout; op.time: new timeout
                    // del old timeout first
                    int pos = 0;
                    if (op.value != timeouts.get(0).time) {
                        pos = findTimeout(op.value);
                    }
                    ValueTime<ArrayList<K>, Long> klt = timeouts.get(pos);
                    klt.value.remove(k);
                    if (klt.value.size() == 0) {
                        timeouts.remove(pos);
                    }
                    // add new timeout
                    insertTimeout(k, op.time);
                }
            }
        // get new min timeout
        if (timeouts.size() > 0) {
            long currNanos = System.nanoTime();
            while (timeouts.size() > 0) {
                ValueTime<ArrayList<K>, Long> kt = timeouts.get(0);
                if (kt.time <= currNanos) {
                    dataLock.lock();
                    try {
                        // this entry already timedout, clean up
                        for (int i = 0; i < kt.value.size(); i++) {
                            data.remove(kt.value.get(i));
                            // //System.out.println("entry timeout before wait: "+kt.value.get(i));
                        }
                    } finally {
                        dataLock.unlock();
                    }
                    timeouts.remove(0);
                } else {
                    nextWakeupTime = kt.time;
                    // System.out.println("start waiting for "+kt.value.get(0));
                    return kt.time - currNanos;
                }
            }
        }
        // if map empty, return max idle time
        nextWakeupTime = Long.MAX_VALUE;
        // System.out.println("start waiting forever");
        return maxSleepTime;
    }

    // use the following method to park worker/purger threads to
    // wait for current timeout or interrupt for update new earliest timeout
    // lock is only hold for condition var wait, released right away after wake
    // up
    // not hold for purger data processing
    // at end of this func, when worker thread wake up,
    // hasNewMin and worker have synced/updated val
    private final long await(long nanoSec) {
        try {
            long remainingNanos = 0;
            wakeupLock.lock();
            if (hasNewMin == true || worker == null) {
                // state changed
                return 0;
            }
            try {
                remainingNanos = wakeupCond.awaitNanos(nanoSec);
            } catch (InterruptedException e) {
                // could happen? who interrupt?
                // we can handle it this way:
                // set worker=-1 here, then at head of loop,
                // if worker==-1, start a new worker thread
                // and then exit loop and this thread.
            }
            return remainingNanos;
        } finally {
            wakeupLock.unlock();
        }
    }

    // worker thread, do:
    // 1. if no entries in map, wait maxIdleMilliSeconds
    // 2. if has entries, find shortest entry timeout and start waiting.
    // 3. when waking up, check in order:
    // 3.0 if worker=null, worker exit at request (such as finalize())
    // 3.1 if hasNewMin=true, waked by signal (a entry with closer timeout
    // inserted, or entry with current timeout removed),
    // change wait on newMinTimeout
    // 3.2 if wait() return > 0, spurious wakeup, wait on remaining time
    // 3.3 wait() returns <= 0, finished timeout:
    // * if map empty,finish waiting idle time,set worker null & exit
    // * del entry corresponding to finished timeout, find new
    // shortest timeout or maxIdleTime if map empty, wait on it.
    @Override
	public void run() {
        System.out.println("worker start");
        // 1. if no entries in map, wait maxIdleMilliSeconds
        // 2. if has entries, find shortest entry timeout and start waiting.
        hasNewMin = true;
        long remainingNanos = 0;
        // wakeup and work cycles:
        while (true) {
            // 3.0 if worker=null, worker wakedup at shutdown (such as
            // finalize())
            if (worker == null) {
                break;
            }
            // 3.1 if hasNewMin=true, waked by signal (a entry with closer
            // timeout
            // inserted, or entry with current timeout removed),
            // change wait on newMinTimeout
            if (hasNewMin) {
                hasNewMin = false;
                remainingNanos = await(updateAndGetMinTimeout());
                continue; // restart checking from loop-top
            }
            // 3.2 if wait() return > 0, spurious wakeup, wait on remaining
            // time
            if (remainingNanos > 0) {
                remainingNanos = await(remainingNanos);
                continue; // restart checking from loop-top
            }
            // 3.3 wait() returns <= 0, finished timeout:
            // * if map empty,finished waiting idle time,set worker null &
            // exit
            if (timeouts.size() == 0) {
                worker = null;
                break;
            }
            // * del entries corresponding to finished timeout, find new
            // shortest timeout or maxIdleTime if map empty, wait on it.
            ValueTime<ArrayList<K>, Long> kt = timeouts.remove(0);
            dataLock.lock();
            try {
                for (int i = 0; i < kt.value.size(); i++) {
                    data.remove(kt.value.get(i));
                    // //System.out.println("entry timeout: "+kt.value.get(i));
                }
            } finally {
                dataLock.unlock();
            }
            remainingNanos = await(updateAndGetMinTimeout());
        }
        // before thread exit, remove shutdown hook
        Runtime.getRuntime().removeShutdownHook(shutdownHook);
        System.out.println("worker exit");
    }
}
