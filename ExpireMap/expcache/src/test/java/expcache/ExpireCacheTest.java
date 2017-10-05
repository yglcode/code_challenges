package expcache;

import static java.lang.System.out;
import static org.junit.Assert.assertEquals;
import static org.junit.Assert.assertTrue;

import java.util.concurrent.CountDownLatch;

import org.junit.After;
import org.junit.Before;
import org.junit.Ignore;
import org.junit.Test;

public class ExpireCacheTest {
	private ExpireCache<String, Integer> em;

	@Before
	public void init() throws Exception {
		System.out.println("Setting up...");
		em = new ExpireCache<String, Integer>();
	}

	@After
	public void fini() throws Exception {
		System.out.println("Tearing down...");
		em.shutdown();
	}

	@Test
	public void testNoExistKey() {
		assertEquals("get Non-exist data return non-null",
				em.get("No such data"), null);
	}

	@Test
	public void testKeyWithZeroLife() {
		em.put("key2", 1, 0); // put data for 0 time, should not add anything
		assertTrue("put() should not add key with 0 timeout", em.size() == 0);
		assertEquals("put() should not add key with 0 timeout",
				em.get("key2"), null);
	}

	@Test
	public void testEntryTimeout() {
		em.put("key1", 1, 200); // put data for 200 millisecs
		assertTrue("put() failed to add key with non-0 timeout", em.size() > 0);
		// verify i can get the value just put in
		assertEquals("get() returns incorrect value", em.get("key1").intValue(), 1);
		// verify i can still get data before timeout
		try {
			Thread.sleep(100);
		} catch (InterruptedException ie) {
			out.println("thread-sleep failed");
			return;
		}
		assertEquals("get() returns incorrect value before timeout", em.get("key1").intValue(), 1);
		// verify i cannot get data after timeout
		try {
			Thread.sleep(300);
		} catch (InterruptedException ie) {
			out.println("thread-sleep failed");
			return;
		}
		assertTrue("entry failed to timeout", em.size() == 0 && em.get("key1") == null);
	}

	@Test
	public void testTimeoutOrder() {
		// check timeout order working correct
		em.put("k1", 1, 250);
		em.put("k2", 2, 300);
		em.put("k3", 3, 100);
		try {
			Thread.sleep(150);
		} catch (InterruptedException ie) {
			out.println("thread-sleep failed");
			return;
		}
		// verify k3 timeout althoug it is added last
		assertTrue("k3 failed to timeout", em.get("k3") == null);
		// check remove works
		em.remove("k2");
		assertTrue("k2 failed to be removed", em.get("k2") == null);
		// after remove k2, i can still get others
		assertTrue("failed to get remaing data(k1)", em.size() > 0 && em.get("k1") == 1);
	}
	
	final static class ParaTest implements Runnable {
		CountDownLatch done;
		ExpireCache<String, Integer> em;
		int start,end;
		public ParaTest(ExpireCache<String, Integer> m, CountDownLatch c, int s, int e) {
			done = c;
			em = m;
			start = s;
			end = e;
		}
		@Override
		public void run() {
			for(int n=start;n<end;n++) {
				em.put(n+"", n, (n-start)+150);
			}
			for(int n=start;n<end;n++) {
				assertTrue("failed to get data", em.get(n+"")!=null);
			}
			for(int n=start;n<end;n++) {
				em.remove(n+"");
			}
			done.countDown();			
		}
	}
	@Test
	public void testParallelAccess() {
		CountDownLatch done = new CountDownLatch(10);
		for(int i=0;i<10;i++) {
			new Thread(new ParaTest(em, done, i*100,(i+1)*100)).start();
		}
		try {
			done.await();
		} catch(InterruptedException ie){}
	}
}