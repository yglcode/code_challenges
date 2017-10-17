package app.server;

import java.io.FileInputStream;
import java.io.IOException;
import java.nio.ByteBuffer;
import java.util.List;
import java.util.Properties;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.ScheduledFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.logging.Logger;

public class AppServer {
	private final static Logger logger = Logger.getLogger(AppServer.class
			.getName());
	public final static byte[] terminateCommand = "terminate".getBytes();
	// only accept numbers of 9 digits, that is from [0, 1 billion)
	public final static int BILLION = 1000000000;
	// server config parameter loaded from properties file
	final static int NUM_DIGITS = 9;
	final static byte[] LineSep = System.lineSeparator().getBytes();
	final static int RECORD_SIZE = NUM_DIGITS + LineSep.length;

	final Config cfg;
	final ExecutorService pool;
	final ScheduledExecutorService sched;
	final ScheduledFuture<?> reporter;
	private final AtomicBoolean active = new AtomicBoolean();

	// use AtomicBitSet (with 1 billion bits) to save if a number has been saved
	final AtomicBitSet savedNumbers = new AtomicBitSet(BILLION);
	// socker connection acceptor
	final Acceptor acceptor;
	// work que of DataProcessors
	final LinkedBlockingQueue<Job> jobQue;
	// queue of ByteBuffers ready to write to log file
	final LinkedBlockingQueue<List<ByteBuffer>> bbufQue;
	// processing status
	final AtomicInteger lastUniqs, lastDups, uniqTotal;
	// data processors
	final DataProcessor[] dataProcs;
	// file writer
	final LogFileWriter logWriter;
	final Thread logWriterThread;

	public AppServer(Properties props) {
		cfg = new Config(props);
		active.set(true);
		pool = Executors.newCachedThreadPool();
		sched = Executors.newScheduledThreadPool(1);
		jobQue = new LinkedBlockingQueue<Job>();
		bbufQue = new LinkedBlockingQueue<List<ByteBuffer>>();
		lastUniqs = new AtomicInteger();
		lastDups = new AtomicInteger();
		uniqTotal = new AtomicInteger();
		// spawn file writer
		logWriter = new LogFileWriter(this);
		logWriterThread = new Thread(logWriter);
		logWriterThread.start();
		// spawn data processors
		dataProcs = new DataProcessor[cfg.numDataProcessors];
		for (int i = 0; i < cfg.numDataProcessors; i++) {
			dataProcs[i] = new DataProcessor(this);
			pool.execute(dataProcs[i]);
		}
		// spawn acceptor
		acceptor = new Acceptor(this);
		pool.execute(acceptor);
		// schedule reporter
		Runnable reportTask = new Runnable() {
			public void run() {
				System.out.println("Received: " + lastUniqs.get()
						+ " unique numbers, " + lastDups.get()
						+ " duplicates. Unique total: " + uniqTotal.get());
				// clear status for last period
				lastUniqs.set(0);
				lastDups.set(0);
			}
		};
		reporter = sched.scheduleAtFixedRate(reportTask, 0,
				cfg.reportIntervalMs, TimeUnit.MILLISECONDS);
		logger.info("AppServer started");
	}

	public void updateStatus(int uniqN, int dupN) {
		lastUniqs.addAndGet(uniqN);
		lastDups.addAndGet(dupN);
		uniqTotal.addAndGet(uniqN);
	}

	public void shutdown() {
		synchronized (this) {
			notifyAll();
		}
	}

	public void close() {
		if (active.get()) {
			active.set(false);
			// close sockets, and so shutdown acceptor and socket handlers
			acceptor.close();
			// first notify log file writer to exit, close fileChan, flush cache
			logWriter.exit.set(true);
			logWriterThread.interrupt();
			// shutdown processors forcely since it is useless anymore
			pool.shutdown(); // Disable new tasks from being submitted
			pool.shutdownNow(); // Cancel currently executing tasks
			// cancel reporter timed tasks
			reporter.cancel(true);
			sched.shutdown();
			sched.shutdownNow();
			// wait for log file writer to finish, so we have clean log file
			try {
				logWriterThread.join();
			} catch (InterruptedException e) {
				try {
					logWriterThread.join();
				} catch (InterruptedException ee) {
				}
			}
			// make sure threadpool threads exit
			pool.shutdownNow();
			try {
				// Wait a while for tasks to respond to being cancelled
				if (!pool.awaitTermination(30, TimeUnit.SECONDS)) {
					System.err.println("Pool did not terminate");
					pool.shutdownNow();
					// Wait a while for tasks to respond to being cancelled
					if (!pool.awaitTermination(60, TimeUnit.SECONDS)) {
						System.err.println("Pool did not terminate");
					}
				}
			} catch (InterruptedException ie) {
				// (Re-)Cancel if current thread also interrupted
				pool.shutdownNow();
				// Preserve interrupt status
				Thread.currentThread().interrupt();
			}
			logger.info("AppServer exited");
		}
	}

	public static void main(String[] args) {
		FileInputStream fis = null;
		AppServer srv = null;
		try {
			fis = new FileInputStream(args[0]);
			Properties props = new Properties();
			props.load(fis);
			srv = new AppServer(props);
		} catch (IOException ios) {
			logger.severe("failed to open server properties file and instantiate server");
			if (fis != null) {
				try {
					fis.close();
				} catch (IOException ioe) {
					logger.warning("failed to close properties file");
				}
			}
			if (srv != null) {
				srv.close();
			}
		}

		// let server run for a while
		synchronized (srv) {
			try {
				srv.wait(srv.cfg.serverRunSec * 1000);
			} catch (InterruptedException e) {
				// TODO Auto-generated catch block
				e.printStackTrace();
			}
		}
		// shutdown
		if (fis != null) {
			try {
				fis.close();
			} catch (IOException ioe) {
				logger.warning("failed to close properties file");
			}
		}
		if (srv != null) {
			srv.close();
		}
	}
}
