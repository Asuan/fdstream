package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime/pprof"
	"sync"

	"github.com/Asuan/fdstream"
)

type context struct {
	isSync  bool
	bind    string
	tcpAddr *net.TCPAddr
}

var (
	ctx        context
	wg         sync.WaitGroup
	logger     *log.Logger
	cpuprofile string
)

//Initialize flags
func Initialize() {
	var (
		err error
	)

	flag.BoolVar(&ctx.isSync, "sync", true, "mode of client")
	flag.StringVar(&ctx.bind, "bind", "0.0.0.0:1900", "address to bind")
	//Profile
	flag.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile `file`")

	flag.Parse()

	if ctx.tcpAddr, err = net.ResolveTCPAddr("tcp", ctx.bind); err != nil {
		fmt.Println("Wrong address: " + err.Error())
	}
	//logger = log.New(ioutil.Discard, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)

	//profile
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		if cpuprofile != "" {
			pprof.StopCPUProfile()
		}
		os.Exit(0)
	}()
	//Profile
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
	}
}

func main() {
	Initialize()

	l, err := net.ListenTCP("tcp", ctx.tcpAddr)
	if err != nil {
		logger.Printf("Could not connect to address: %s error: %v", ctx.tcpAddr.String(), err)
	}
	defer l.Close()
	var i int
	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			logger.Printf("Ooops comunication going wrong |-) error: %v", err)
			break
		}
		//Tune system buffer
		conn.SetReadBuffer(fdstream.MaxMessageSize * 200)
		conn.SetWriteBuffer(fdstream.MaxMessageSize * 200)
		conn.SetKeepAlive(true)

		//Example handlers for connections
		go HandleTCP(conn, i)

		i++
	}

	logger.Printf("Stop server server")

}

//HandleTCP handle messages
func HandleTCP(conn *net.TCPConn, instanceNum int) error {
	defer conn.Close()

	logger.Printf("Handle connection: %s", conn.RemoteAddr().String())

	var (
		i   int
		err error
		wg  sync.WaitGroup
		cl  *fdstream.AsyncClient
	)
	cl, err = fdstream.NewAsyncHandler(conn, conn)
	if err != nil {
		return err
	}

	//Example worker for handling income messages
	worker := func(async *fdstream.AsyncClient) {
		var (
			message     *fdstream.Message
			backPayload []byte
		)
		for async.IsAlive() {
			i++
			backPayload = []byte(fmt.Sprintf("Responce I-%d-#%d", instanceNum, i))
			message = async.Read() //It is thread safe to read and write
			logger.Printf("Get message %s", message.Name)

			async.Write(&fdstream.Message{
				Id:      message.Id,   //It is need for sync communication
				Name:    message.Name, //Same name for validating
				Payload: backPayload,
			})
		}
		wg.Done()
	}

	wg.Add(2)
	go worker(cl)
	go worker(cl)
	wg.Wait()

	logger.Printf("Finish serving connection %d with total messages count: %d", instanceNum, i)
	return nil
}
