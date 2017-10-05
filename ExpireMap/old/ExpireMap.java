import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.concurrent.locks.Condition;
import java.util.concurrent.locks.ReentrantLock;
import static java.lang.System.out;

/*
 * Requirements of ExpireMap:
 * 1. thread safe.
 * 2. only take space proportional to the current number of entries in it.
 * 3. timeout should be enforced as accurately as the underlying operating system allows.
 * 4. be efficient in the big O time of each of three methods in the interface.
 */
interface ExpireMapIntf<K,V> {
	//If there is no entry with the key in the map, add the key/value pair as a new entry
	//If there is an existing entry with the key, the current entry will be replaced with the new key/value pair.
	//If the newly added entry is not removed after timeoutMs since it's added to the map, remove it.
	//assuming timeout is specified in milliseconds.
	void put(K key, V value,long timeoutMs);
	
	//Get the value associated with the key if present; otherwise, return null.
	V get(K key);
	
	//Remove the entry associated with key, if any.
	void remove(K key);
	
	//return map size
	int size();
}

//helper class to store a pair value
class ValueTime<S, T extends Comparable<T>> implements Comparable<ValueTime<S,T>> {
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
 * ExpireMap implementation notes:
 * 1. Use a HashMap to store entries.
 * 2. Use a dynamic array (ArrayList) to store sorted timeouts,
 * 		backed up by timeout cache: timeoutToAddDel processed by worker thread
 * 3. Use a worker thread to update timeouts list and purge timeouted entries.
 * 4. off-load time-consuming tasks to worker thread: insert new timeout, find timeout O(log)
 * 5. all 3 API methods only involve constant time operations O(1)
 * 6. only wake up worker thread when has new shortest timeout or too much timeout pending.
 * 7. Use nano seconds for better precision.
 * 8. Use ReentrantLock to protect thread safety.
 * 9. Use Condition for worker thread to wait for timeout.
 * 10. make worker daemon thread and add Runtime.shutdownHook to clean it.
 */
public class ExpireMap<K,V> implements ExpireMapIntf<K,V>, Runnable {
	//configurations:
	//how long to keep worker thread when no activity;
	//after this, worker thread will exit until ExpireMap is active again.
	private final static long DEFAULT_MAX_IDLE_TIME = 3000; //default to 3 seconds
	private final long maxIdleMilliSeconds;
	//limit too much pending timeouts to add & del
	private final static int TimeoutsToAddDelFactor = 2; //cached timeouts <= 2*map.szie()
	private final static int TimeoutsToAddDelMax0 = 32; //or <=32
	
	//data stores inserted (key,val) pair and its timeout.
	private final HashMap<K, ValueTime<V,Long>> data = new HashMap<K,ValueTime<V,Long>>();
	//timeouts - the list of (entry-key,timeout) pair sorted by timeout.
	//keep it sorted,inserted by binary search; so it is O(log) operations;
	//not O(1) as hashmap; so let worker thread maintain it from ToAddDel list.
	//Use ArrayList to store keys with same timeout could be overkill/overhead,
	//but in case some machine don't provide precise enough timing, entries could
	//end up with same timestamp.
	private final ArrayList<ValueTime<ArrayList<K>,Long>> timeouts = new ArrayList<ValueTime<ArrayList<K>,Long>>();
	//cached timeouts to be added/deled to timeouts list by worker thread
	//put,remove will only signal/wake worker thread when new timeout is closer or too much timeouts pending.
	//kept in incoming order
	private final ArrayList<ValueTime<Boolean,ValueTime<K,Long>>> timeoutsToAddDel = new ArrayList<ValueTime<Boolean,ValueTime<K,Long>>>();
	//Lock to protect thread safety
	private final ReentrantLock lock = new ReentrantLock();
	//cond var to allow purge thread wait
	private final Condition cond = lock.newCondition();
	//shutdown hook for clean up worker thread when JVM exit
	private final Thread shutdownHook = new Thread(
			new Runnable() {
				@Override
				public void run() {
					shutdown();
				}
			});
	
	//do i have a worker thread
	private Thread worker;
	//do i have a new min timeout, one of states when waking up worker thread
	private boolean hasNewMin;
	
	public ExpireMap() {
		this(DEFAULT_MAX_IDLE_TIME);
	}

	public ExpireMap(long maxIdleMilliSec) {
		maxIdleMilliSeconds = maxIdleMilliSec;
	}

	@Override
	protected void finalize() throws Throwable {
		super.finalize();
		shutdown();
	}
	
	//shutdown worker thread if it still there
	public void shutdown() {
		System.out.println("shutdown called");
		try {
			lock.lock();
			if (worker != null) {
				worker = null; //tell active worker thread exit
				cond.signalAll();
			}
		}
		finally {
			lock.unlock();
		}		
	}

	//assuming timeout is specified in millisecond
	@Override
	public void put(K key, V value, long timeoutMs) {
		if (timeoutMs <= 0) {
			return;
		}
		try {
			lock.lock();
			//check if worker running, otherwise starts it
			if (worker == null) {
				Runtime.getRuntime().addShutdownHook(shutdownHook);
				worker = new Thread(this);
				worker.setDaemon(true); //so jvm can exit without waiting for worker
				worker.start();			
			}
			//check if k already exists inside map, clean up
			ValueTime<V,Long> vt = data.remove(key);
			if(vt != null) {
				//cache del-timeout events for worker thread to process
				timeoutsToAddDel.add(new ValueTime<Boolean, ValueTime<K, Long>>(false,new ValueTime<K,Long>(key,vt.time)));
				//if this entry is the shortest worker currently wait on,
				//wake up worker thread and let it recheck.
				if (vt.time == timeouts.get(0).time) {
					hasNewMin = true;
					cond.signalAll();
				}
			}
			//add new entry
			long newTO = System.nanoTime() + timeoutMs * (long)1e6;
			data.put(key, new ValueTime<V,Long>(value, newTO));
			//cache add-timeout events for worker thread to process
			timeoutsToAddDel.add(new ValueTime<Boolean, ValueTime<K, Long>>(true,new ValueTime<K, Long>(key,newTO)));
			//if this entry has shorter timeout than worker currently wait on,
			//wake up worker thread and let it recheck.
			int cacheLimit = TimeoutsToAddDelFactor*timeouts.size();
			cacheLimit = (cacheLimit > TimeoutsToAddDelMax0)?cacheLimit:TimeoutsToAddDelMax0;
			if ((timeouts.size()==0) //worker waiting in idle
					|| (timeouts.size() > 0 && newTO < timeouts.get(0).time) //entry has shorter timeout than worker currently wait on
					|| timeoutsToAddDel.size() > cacheLimit//avoid pending timeouts take unlimited space
					) {
				hasNewMin = true;
				cond.signalAll();
			}
		}
		finally {
			lock.unlock();
		}		
	}

	@Override
	public V get(K key) {
		try {
			lock.lock();
			ValueTime<V,Long> vt = data.get(key);
			if (vt == null) {
				return null;
			}
			return vt.value;
		}
		finally {
			lock.unlock();
		}
	}

	@Override
	public void remove(K key) {
		try {
			lock.lock();
			ValueTime<V,Long> vt = data.remove(key);
			if (vt == null) {
				return;
			}
			//cache del-timeout events for worker thread to process
			timeoutsToAddDel.add(new ValueTime<Boolean, ValueTime<K, Long>>(false,new ValueTime<K, Long>(key,vt.time)));
			//if worker currently waiting on this key, wakeup it
			int cacheLimit = TimeoutsToAddDelFactor*timeouts.size();
			cacheLimit = (cacheLimit > TimeoutsToAddDelMax0)?cacheLimit:TimeoutsToAddDelMax0;
			if (vt.time == timeouts.get(0).time ||
					timeoutsToAddDel.size() > cacheLimit//avoid pending timeouts take unlimited space
					) {
				hasNewMin = true;
				cond.signalAll();
			}
		}
		finally {
			lock.unlock();
		}		
	}
	
	@Override
	public int size() {
		try {
			lock.lock();
			return data.size();
		}
		finally {
			lock.unlock();
		}				
	}
	
	//binary search timeout entry
	//always called from worker thread inside lock
	private int findTimeout(ValueTime<K,Long> keyTime) {
		return Collections.binarySearch(timeouts, new ValueTime<ArrayList<K>,Long>(null,keyTime.time));
	}
	
	//insert timeout by binary search
	//always called from worker thread inside lock
	private void insertTimeout(ValueTime<K,Long> keyTime) {
		ValueTime<ArrayList<K>,Long> klTime = new ValueTime<ArrayList<K>,Long>(null,keyTime.time);
		int pos = Collections.binarySearch(timeouts, klTime);
		if (pos < 0) {
			klTime.value = new ArrayList<K>();
			klTime.value.add(keyTime.value);
			timeouts.add(-1-pos, klTime);
			//System.out.println("add1 "+keyTime.value);
		} else {
			//this case could happen if machine don't provide enough time precision.
			//some entries could end up with same timeout value.
			ValueTime<ArrayList<K>,Long> klt = timeouts.get(pos);
			klt.value.add(keyTime.value);
			//System.out.println("add2 "+keyTime.value);
		}
	}
	
	//get new min timeout for worker to wait
	//always called from worker thread inside lock
	private long getMinTimeout() {
		//apply cached timeout operations
		int num = timeoutsToAddDel.size();
		for(int i=0;i<num;i++) {
			//apply timeout changes according to its timing order.
			ValueTime<Boolean,ValueTime<K,Long>> op = timeoutsToAddDel.remove(0);
			if (op.value) { //add a timeout
				insertTimeout(op.time);
			} else { //del timeout
				ValueTime<K,Long> keyTime = op.time;
				int pos = 0;
				if (keyTime.time != timeouts.get(0).time) {
					pos = findTimeout(keyTime);
				}
				ValueTime<ArrayList<K>,Long> klt = timeouts.get(pos);
				klt.value.remove(keyTime.value);
				if (klt.value.size()==0) {
					timeouts.remove(pos);
				}
			}
		}
		//get new min timeout
		if (timeouts.size() > 0) {
			long currNanos = System.nanoTime();
			while(timeouts.size() > 0) {
				ValueTime<ArrayList<K>,Long> kt = timeouts.get(0);
				if (kt.time <= currNanos) {
					//this entry already timedout, clean up
					for(int i=0;i<kt.value.size();i++) {
						data.remove(kt.value.get(i));
						//System.out.println("entry timeout before wait: "+kt.value.get(i));
					}
					timeouts.remove(0);
				} else {
					return kt.time - currNanos;
				}
			}
		}
		//if map empty, return max idle time
		return maxIdleMilliSeconds * (long)1e6;
	}
	
	//worker thread, do:
	//1. if no entries in map, wait maxIdleMilliSeconds
	//2. if has entries, find shortest entry timeout and start waiting.
	//3. when waking up, check in order:
	//	 3.0 if worker=null, worker exit at request (such as finalize())
	//	 3.1 if hasNewMin=true, waked by signal (a entry with closer timeout 
	//			inserted, or entry with current timeout removed),
	//			change wait on newMinTimeout
	//   3.2 if wait() return > 0, spurious wakeup, wait on remaining time
	//	 3.3 wait() returns <= 0, finished timeout:
	//		  	* if map empty,finish waiting idle time,set worker null & exit
	//			* del entry corresponding to finished timeout, find new
	//				shortest timeout or maxIdleTime if map empty, wait on it.
	@Override
	public void run() {
		//System.out.println("worker start");
		try {
			lock.lock();
			// 1. if no entries in map, wait maxIdleMilliSeconds
			// 2. if has entries, find shortest entry timeout and start waiting.
			hasNewMin = true;
			long remainingNanos = 0;
			//wakeup and work cycles:
			while (true) {
				//3.0 if worker=null, worker wakedup at shutdown (such as finalize())
				if (worker == null) {
					break;
				}
				//3.1 if hasNewMin=true, waked by signal (a entry with closer timeout 
				//	inserted, or entry with current timeout removed),
				//	change wait on newMinTimeout
				if (hasNewMin) {
					hasNewMin = false;
					try {
						remainingNanos = cond.awaitNanos(getMinTimeout());
					} catch (InterruptedException e) {
						//could happen? who interrupt?
						//we can handle it this way: 
						//set worker=-1 here, then at head of loop, 
						//if worker==-1, start a new worker thread 
						//and then exit loop and this thread.
					}
					continue; //restart checking from loop-top
				}
				//3.2 if wait() return > 0, spurious wakeup, wait on remaining time
				if (remainingNanos > 0) {
					try {
						remainingNanos = cond.awaitNanos(remainingNanos);
					} catch (InterruptedException e) {
						//could happen? who interrupt?
						//we can handle it this way: 
						//set worker=-1 here, then at head of loop, 
						//if worker==-1, start a new worker thread 
						//and then exit loop and this thread.
					}
					continue; //restart checking from loop-top					
				}
				//3.3 wait() returns <= 0, finished timeout:
				//* if map empty,finished waiting idle time,set worker null & exit
				if (timeouts.size() == 0) {
					worker = null;
					break;
				}
				//* del entries corresponding to finished timeout, find new
				//		shortest timeout or maxIdleTime if map empty, wait on it.
				ValueTime<ArrayList<K>,Long> kt = timeouts.remove(0);
				for(int i=0;i<kt.value.size();i++) {
					data.remove(kt.value.get(i));
					//System.out.println("entry timeout: "+kt.value.get(i));
				}
				try {
					remainingNanos = cond.awaitNanos(getMinTimeout());
				} catch (InterruptedException e) {
					//could happen? who interrupt?
					//we can handle it this way: 
					//set worker=-1 here, then at head of loop, 
					//if worker==-1, start a new worker thread 
					//and then exit loop and this thread.
				}				
			}
			//before thread exit, remove shutdown hook
			Runtime.getRuntime().removeShutdownHook(shutdownHook);
		}
		finally {
			lock.unlock();
		}					
		System.out.println("worker exit");
	}
	
	//--- unit tests ----
	
	//test map put/get correctly
	public static void unitTests(ExpireMap<String,Integer> em) {
		//get non-exit key
		int failed = 0;
		if (em.get("No such data") != null) {
			out.println("failed: get Non-exit data return non-null");
			failed++;
		}
		em.put("key2",1,0); //put data for 0 time, should not add anything
		if (em.size()>0 || em.get("key2") != null) {
			out.println("failed: put() shoudl not add entry with 0 timeout.");
			failed++;
		}
		em.put("key1",1,200); //put data for 200 millisecs
		if (em.size()==0) {
			out.println("failed: put() failed to add data");
			failed++;			
		}
		//verify i can get the value just put in
		if (em.get("key1") != 1) {
			out.println("failed: get() return incorrect data");
			failed++;
		}
		//verify i can still get data before timeout
		try {Thread.sleep(100);}catch(InterruptedException ie){out.println("thread-sleep failed");return;}
		if (em.get("key1") != 1) {
			out.println("failed: get() return incorrect data");
			failed++;
		}
		//verify i cannot get data after timeout
		try {Thread.sleep(300);}catch(InterruptedException ie){out.println("thread-sleep failed");return;}
		if (em.size()>0 || em.get("key1") != null) {
			out.println("failed: put-in value should have timeouted.");
			failed++;
		}
		//check timeout order working correct
		em.put("k1",1, 250);
		em.put("k2",2, 300);
		em.put("k3",3, 100);
		try {Thread.sleep(150);}catch(InterruptedException ie){out.println("thread-sleep failed");return;}
		//verify k3 timeout althoug it is added last
		if (em.get("k3") != null) {
			out.println("failed: get() return timeouted data");
			failed++;
		}
		em.remove("k2");
		//check remove works
		if(em.get("k2") != null) {
			out.println("failed: get() return removed data");
			failed++;			
		}
		//after remove k2, i can still get others
		if(em.size() == 0 || em.get("k1") != 1) {
			out.println("failed: get() return incorrect data");
			failed++;			
		}
		//done all cases
		if (failed == 0) {
			out.println("all test cases passed");
		}
	}

	public static void main(String[] args) {
		System.out.println("hi");
		ExpireMap<String,Integer> em1 = new ExpireMap<String,Integer>();
		unitTests(em1);
		//try {Thread.sleep(3500);}catch(InterruptedException ie){out.println("thread-sleep failed");return;}
		System.out.println("done");
	}

}

