package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"github.com/Asuan/fdstream"
)

type context struct {
	isSync  bool
	bind    string
	tcpAddr *net.TCPAddr
}

var (
	ctx    context
	wg     sync.WaitGroup
	logger *log.Logger
)

//Initialize flags
func Initialize() {
	var (
		err error
	)
	flag.BoolVar(&ctx.isSync, "sync", true, "mode of client")
	flag.StringVar(&ctx.bind, "bind", "0.0.0.0:1900", "address to bind")
	flag.Parse()

	if ctx.tcpAddr, err = net.ResolveTCPAddr("tcp", ctx.bind); err != nil {
		fmt.Println("Wrong address: " + err.Error())
	}
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
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
		go HandleTCP(conn, i)
		i++
	}

	logger.Printf("Stop server server")
}

//HandleTCP some test worker for communication
func HandleTCP(conn *net.TCPConn, instanceNum int) {
	defer conn.Close()
	conn.SetReadBuffer(fdstream.MaxMessageSize * 20)
	conn.SetWriteBuffer(fdstream.MaxMessageSize * 20)
	conn.SetKeepAlive(true)
	logger.Printf("Handle connection: %s", conn.RemoteAddr().String())
	cl, err := fdstream.NewAsyncHandler(conn, conn)

	var (
		i int
	)
	for cl.IsAlive() {
		i++

		message := cl.Read()
		logger.Printf("Get message")
		if err != nil {
			logger.Printf("Error respoonce: %v\n", err)
		}
		message.Payload = []byte(fmt.Sprintf("Responce I-%d-#%d", instanceNum, i))
		cl.Write(message)
	}

	logger.Printf("Finish serving connection %d with total messages count: %d", instanceNum, i)
}
