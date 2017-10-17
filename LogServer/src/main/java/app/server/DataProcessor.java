package app.server;

import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.channels.SocketChannel;
import java.util.ArrayList;
import java.util.List;
import java.util.logging.Logger;

class Job {
	public final SocketChannel sockCh;
	public final List<byte[]> data;

	public Job(SocketChannel sc) {
		sockCh = sc;
		data = new ArrayList<byte[]>();
	}
}

public class DataProcessor implements Runnable {
	private final static Logger logger = Logger.getLogger(DataProcessor.class
			.getName());
	private final AppServer srv;

	public DataProcessor(AppServer s) {
		srv = s;
	}

	@Override
	public void run() {
		try {
			boolean done = false;
			while (!done) {
				Job job = srv.jobQue.take();
				done = process(job);
			}
		} catch (InterruptedException ie) {

		}
		logger.info("1 DataProcessor exit");
	}

	private boolean process(Job job) {
		int numNew = 0, numDup = 0;
		List<ByteBuffer> bufsToWrite = new ArrayList<ByteBuffer>();
		boolean inputError = false;
		processLoop: for (int k = 0; k < job.data.size() && !inputError; k++) {
			byte[] data = job.data.get(k);
			int idx0 = 0;
			for (int i = 0; i < data.length && !inputError; i += AppServer.RECORD_SIZE) {
				// first check we have line-sep at right spot
				for (int j = 0; j < AppServer.LineSep.length; j++) {
					if (data[i + AppServer.NUM_DIGITS + j] != AppServer.LineSep[j]) {
						inputError = true;
						break processLoop;
					}
				}
				// check if we are terminated
				if (data[i] < '0' || data[i] > '9') {
					boolean terminated = true;
					for (int j = 0; j < AppServer.NUM_DIGITS; j++) {
						if (data[i + j] != AppServer.terminateCommand[j]) {
							terminated = false;
							break;
						}
					}
					if (terminated) {
						srv.shutdown();
						return true;
					} else {
						inputError = true;
						break processLoop;
					}
				}
				// make sure we have 9 valid digits
				int num = 0;
				for (int j = 0; j < AppServer.NUM_DIGITS; j++) {
					byte digit = data[i + j];
					if (digit < '0' || digit > '9') {
						inputError = true;
						break processLoop;
					}
					num = num * 10 + digit - '0';
				}
				// if dup, skip it
				boolean skip = true;
				if (srv.savedNumbers.get(num) == true) {
					numDup++;
				} else {
					if (srv.savedNumbers.set(num)) {
						// i set the number successfully, i save it to file
						numNew++;
						skip = false;
					} else {
						// others set the number, they will save it
						numDup++;
					}
				}
				if (skip) {
					// logger.info("skip: segment["+idx0+","+(i - idx0)+"]");
					if (i > idx0) {
						bufsToWrite
								.add(ByteBuffer.wrap(data, idx0, (i - idx0)));
					}
					idx0 = i + AppServer.RECORD_SIZE;
				}
			}
			// logger.info("add segment["+idx0+","+data.length+"]");
			if (data.length > idx0) {
				bufsToWrite.add(ByteBuffer.wrap(data, idx0,
						(data.length - idx0)));
			}
		}
		// push cached result
		if (bufsToWrite.size() > 0) {
			try {
				srv.bbufQue.put(bufsToWrite);
			} catch (InterruptedException e) {
			}
		}
		// close input socket channel to allow socket handler exit
		if (inputError) {
			try {
				job.sockCh.close();
			} catch (IOException ioe) {

			}
		}
		// update server status
		srv.updateStatus(numNew, numDup);
		return false;
	}

}
