package app.server;

import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.channels.SocketChannel;
import java.util.logging.Logger;

public class SocketHandler implements Runnable {
	private final static Logger logger = Logger.getLogger(SocketHandler.class
			.getName());
	private final SocketChannel sockCh;
	private final AppServer srv;

	public SocketHandler(SocketChannel sock, AppServer as) {
		sockCh = sock;
		srv = as;
	}

	@Override
	public void run() {
		ByteBuffer buf = ByteBuffer.allocateDirect(srv.cfg.sockChanRecvBufSiz);
		Job job = new Job(sockCh);
		try {
			while (sockCh.read(buf) >= 0 || buf.position() > 0) {
				buf.flip();
				int len = buf.remaining();
				if (len < AppServer.RECORD_SIZE
						&& len >= AppServer.LineSep.length) {
					// check if we got shorter line
					int pos = len;
					for (int j = 0; j < AppServer.LineSep.length; j++) {
						if (AppServer.LineSep[AppServer.LineSep.length - 1 - j] != buf
								.get(pos - 1 - j)) {
							pos = -1;
							break;
						}
					}
					if (pos > 0) {// shorter line, invalid input
						logger.warning("invalid input on socket");
						break;
					}
				} else {
					// find the last valid newline
					int pos = (len / AppServer.RECORD_SIZE)
							* AppServer.RECORD_SIZE;
					// should fine new line at end, otherwise fail
					for (int j = 0; j < AppServer.LineSep.length; j++) {
						if (AppServer.LineSep[AppServer.LineSep.length - 1 - j] != buf
								.get(pos - 1 - j)) {
							pos = -1;
							break;
						}
					}
					if (pos < 0) {// invalid input
						logger.warning("invalid input on socket");
						break;
					}
					byte[] data = new byte[pos];
					buf.get(data);
					job.data.add(data); // add data to job buffer
					// submit job to DataProcessor
					if (srv.jobQue.offer(job)) {
						// successfully submit job, create a new job to buffer
						// new data
						job = new Job(sockCh);
					} // otherwise continue appending byte[] data to current job
				}
				buf.compact();
			}
		} catch (IOException ios) {
			logger.warning("socket channel closed");
		} finally {
			try {
				sockCh.close();
			} catch (IOException ioe) {

			}
		}
		srv.acceptor.handlerFinish(sockCh);
		logger.info("1 socket handler exits");
	}

}
