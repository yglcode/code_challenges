package main
import (
	"log"
	"flag"
	"net"
	"strconv"
	"os"
	"os/signal"
	"syscall"
	"sync"
	"time"
)

//config
var (
	port int
	backupInterval int
	backupDir string
)
func init() {
	flag.IntVar(&port, "port", 8080, "port index server listen at")
	flag.StringVar(&backupDir, "backupDir", "./pkgs_registry/", "port index server listen at")
	flag.IntVar(&backupInterval, "backupInterval", 30, "seconds before next backup of package index")
}

func main() {
	flag.Parse()

	//start listening
	l, err := net.Listen("tcp",":"+strconv.Itoa(port))
	//l, err := net.ListenTCP("tcp",&TCPAddr{Port:port})
	if err != nil {
		log.Fatalf("Listen failed: %v", err)
	}
	defer l.Close()

	//set handlers for signal interrupt & terminate
	sigCh := make(chan os.Signal,1)
	signal.Notify(sigCh, syscall.SIGTERM)
	signal.Notify(sigCh, syscall.SIGINT)

	//create the single service instance
	srv,err := NewServer(backupDir)
	if err != nil {
		log.Fatalln("Failed to create server:",err)
	}

	//close stopChan to notify all handlers to exit
	stopChan := make(chan struct{})
	//use wait group to wait for all sub goroutines to finish
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Println("Server exiting: ",err)
				srv.backupRegistry()
				break
			}
			wg.Add(1) //make sure server wait for all handlers
			go func() {
				defer wg.Done()
				srv.HandleConn(conn,stopChan)
			}()
		}
	}()

	//time ticker to trigger registry backup
	tick := time.NewTicker(time.Duration(backupInterval) * time.Second)
MainLoop:
	for {
		select {
		case _ = <-tick.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				srv.backupRegistry()
			}()
		case sig := <- sigCh:
			log.Println("Receive signal: ",sig)
			l.Close() //notify accept goroutine exit
			tick.Stop()
			close(stopChan) //notify handler goroutines exit
			break MainLoop
		}
	}
	//wait for all sub goroutines finish
	wg.Wait()
}
