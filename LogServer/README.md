Requirements:
=============

Write a server (“Application”) in Java that opens a socket and restricts input to at most 5 concurrent clients. Clients will connect to the Application and write any number of 9 digit numbers, and then close the connection. The Application must write a de-duplicated list of these numbers to a log file in no particular order.
Close connection if see invalid data.
Shutdown when see "terminate" command.

Implementation:
===============

 1. use nio SocketChannel/FileChannel/ByteBuffer for io, not using Selector/epoll since we are handling small number of clients and we need max throughput.

 2. since 9 digit numbers are in range [0, 1 billion), use a AtomicBitSet based on AtomicIntegerArray to keep which numbers have been recorded and save memory.

 3. architecture:

    . one acceptor thread, accept client connections and dispatch handler threads.

    . one handler thread for each connected socket, receive data in bytebuffer and forward to processor threads.

    . a few processors threads (number configurable), process received bytebuffer, check invalid input, check termination signal, check if number is duplicated, and then forward valid data to log file writer for output.

    . most threads running inside threadpool, only log file writer thread running separately, since during shutdown, we need wait/join it so that we are sure file has been flushed/closed cleanly.

 4. performance consideration:

    . use large socket recv buffer and socket channel recv ByteBuffer (100k) for high throughput (configurable through properties file).

    . use Gather-writing to write a group of ByteBuffers to log file.

    . avoid copying received data, create output ByteBuffers directly from received ones.

    . one log file writer thread open file and keep appending data in large buffer.

Build & Run:
============

mvn package

java -cp target/AppServer-1.0-SNAPSHOT.jar app.server.AppServer config/AppServer.props


Configuration (config/AppServer.props):
=======================================

# server listening port
listener.port=4000
# max concur client conns, default 5
num.concur.clients=5
# log file path name
log.file.path.name=/tmp/numbers.log
# log file batch size, 32k
log.file.batch.size=32768
# socket receive buffer size, 100k
socket.receive.buffer.bytes=102400
# ByteBuffer size to receive from SocketChannel, 100k
socket.channel.receive.buffer.bytes=102400
# report interval, 10 sec 
report.interval.ms = 10000

