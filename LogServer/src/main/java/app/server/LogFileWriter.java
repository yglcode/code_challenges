package app.server;

import java.io.IOException;
import java.nio.ByteBuffer;
import java.nio.channels.FileChannel;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.StandardOpenOption;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.logging.Logger;

public class LogFileWriter implements Runnable {
	private final static Logger logger = Logger.getLogger(LogFileWriter.class
			.getName());
	private final AppServer srv;
	private FileChannel fout;
	final AtomicBoolean exit = new AtomicBoolean();

	public LogFileWriter(AppServer s) {
		srv = s;
		try {
			Path path = Paths.get(srv.cfg.logFile);
			fout = FileChannel.open(path, StandardOpenOption.CREATE,
					StandardOpenOption.WRITE,
					StandardOpenOption.TRUNCATE_EXISTING);
			logger.info("log file created");
		} catch (IOException e) {
			e.printStackTrace();
		}
	}

	@Override
	public void run() {
		List<ByteBuffer> bbufs = new ArrayList<ByteBuffer>();
		logger.info("log writer started");
		try {
			while (exit.get() == false) {
				int batchSize = 0;
				try {
					bbufs = srv.bbufQue.take();
				} catch (InterruptedException e) {
					logger.warning(e.getMessage());
					continue; // go check if exit flag set
				}
				logger.info("recv " + bbufs.size() + " bytebuffers");
				// accum batch size
				for (ByteBuffer bb : bbufs) {
					batchSize += bb.remaining();
				}
				// combine remaining list
				while (batchSize < srv.cfg.logFileBatchSize) {
					List<ByteBuffer> bbs = srv.bbufQue.poll();
					if (bbs == null) {
						break;
					}
					for (ByteBuffer bb : bbs) {
						batchSize += bb.remaining();
					}
					bbufs.addAll(bbs);
				}
				if (batchSize > 0) {
					logger.info("write " + batchSize + " bytes to log file");
					fout.write(bbufs.toArray(new ByteBuffer[bbufs.size()]));
				}
			}
		} catch (IOException e) {
			logger.warning(e.getMessage());
		} finally {
			try {
				fout.close();
			} catch (IOException e) {
				// TODO Auto-generated catch block
				e.printStackTrace();
			}
		}
		logger.info("Number log writer exit");
	}
}