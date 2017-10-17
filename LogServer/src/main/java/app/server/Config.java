package app.server;

import java.util.Properties;

public class Config {
	public int listenPort = 9876;
	public int maxClients = 5;
	public String logFile = "/tmp/numbers.log";
	public int logFileBatchSize = 32768; // 32k
	public int sockRecvBufSiz = 102400; // 100k
	public int sockChanRecvBufSiz = 102400; // 100k
	public long reportIntervalMs = 10000; // 10sec for each report
	public int numDataProcessors = 4;
	public int serverRunSec = 30; // run server for 10 sec

	public Config(Properties props) {
		String port = props.getProperty("listener.port");
		if (port != null) {
			listenPort = Integer.parseInt(port);
		}
		String mc = props.getProperty("num.concur.clients");
		if (mc != null) {
			maxClients = Integer.parseInt(mc);
		}
		String logdir = props.getProperty("log.file.path.name");
		if (logdir != null) {
			logFile = logdir;
		}
		String lfbs = props.getProperty("log.file.batch.size");
		if (lfbs != null) {
			logFileBatchSize = Integer.parseInt(lfbs);
		}
		String srbs = props.getProperty("socket.receive.buffer.bytes");
		if (srbs != null) {
			sockRecvBufSiz = Integer.parseInt(srbs);
		}
		String scrbs = props.getProperty("socket.channel.receive.buffer.bytes");
		if (scrbs != null) {
			sockChanRecvBufSiz = Integer.parseInt(scrbs);
		}
		String rim = props.getProperty("report.interval.ms");
		if (rim != null) {
			reportIntervalMs = Long.parseLong(rim);
		}
		String ndp = props.getProperty("num.data.processors");
		if (ndp != null) {
			numDataProcessors = Integer.parseInt(ndp);
		}
		String srds = props.getProperty("server.run.durationSec");
		if (srds != null) {
			serverRunSec = Integer.parseInt(srds);
		}
	}
}
