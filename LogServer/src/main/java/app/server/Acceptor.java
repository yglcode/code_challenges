package app.server;

import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.StandardSocketOptions;
import java.nio.channels.ServerSocketChannel;
import java.nio.channels.SocketChannel;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.Semaphore;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.logging.Logger;

public class Acceptor implements Runnable {
	private final static Logger logger = Logger.getLogger(Acceptor.class
			.getName());
	private final AppServer srv;
	private final Semaphore sema;
	private final List<SocketChannel> actSockets = new ArrayList<SocketChannel>();
	private final AtomicBoolean active = new AtomicBoolean();
	private ServerSocketChannel serverSock;

	public Acceptor(AppServer as) {
		srv = as;
		sema = new Semaphore(srv.cfg.maxClients, false);
	}

	@Override
	public void run() {
		try {
			serverSock = ServerSocketChannel.open();
			active.set(true);
			serverSock.bind(new InetSocketAddress(srv.cfg.listenPort));
			serverSock.setOption(StandardSocketOptions.SO_REUSEADDR, true);
			for (;;) {
				try {
					sema.acquire();
				} catch (InterruptedException ie) {
					logger.warning("acceptor waiting on semaphere is interrupted");
					continue;
				}
				SocketChannel sock = serverSock.accept();
				// set default recv buf size for socket channel recved from here
				sock.setOption(StandardSocketOptions.SO_RCVBUF,
						srv.cfg.sockRecvBufSiz);
				SocketHandler hdlr = new SocketHandler(sock, srv);
				synchronized (actSockets) {
					actSockets.add(sock);
				}
				srv.pool.execute(hdlr);
			}
		} catch (IOException ioe) {
			logger.warning("acceptor thread exist with exception: "
					+ ioe.getMessage());
		} finally {
			try {
				serverSock.close();
			} catch (IOException ioe) {

			}
		}
		active.set(false);
		srv.shutdown();
		logger.info("acceptor exits");
	}

	public void handlerFinish(SocketChannel sock) {
		sema.release();
		synchronized (actSockets) {
			actSockets.remove(sock);
		}
	}

	public void close() {
		if (active.get()) {
			active.set(false);
			try {
				// shutdown acceptor
				serverSock.close();
				// if acceptor blocking waiting for handlers to exit, unblock it
				sema.release();
				// tell handlers to exit
				synchronized (actSockets) {
					for (SocketChannel sock : actSockets) {
						sock.close();
					}
				}
			} catch (IOException ioe) {

			}
		}
	}
}
