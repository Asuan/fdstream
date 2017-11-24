package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Asuan/fdstream"
)

type context struct {
	isSync  bool
	server  string
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
	flag.StringVar(&ctx.server, "server", "0.0.0.0:1900", "address of server")
	flag.Parse()

	if ctx.tcpAddr, err = net.ResolveTCPAddr("tcp", ctx.server); err != nil {
		fmt.Println("Wrong address: " + err.Error())
	}
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
}

func main() {
	Initialize()

	conn, err := net.DialTCP("tcp", nil, ctx.tcpAddr)
	if err != nil {
		logger.Printf("Could not connect to address: %s error: %v", ctx.tcpAddr.String(), err)
	}
	defer conn.Close()
	conn.SetReadBuffer(fdstream.MaxMessageSize * 20)
	conn.SetWriteBuffer(fdstream.MaxMessageSize * 20)
	conn.SetKeepAlive(true)
	cl, err := fdstream.NewSyncClient(conn, conn, time.Duration(2*time.Minute))
	if err != nil {
		logger.Printf("Could not create instance %v", err)
	}
	wg.Add(60)
	for i := 0; i < 60; i++ {
		logger.Printf("Start client %d", i)
		go HandlerClient(cl, i)
	}
	wg.Wait()

	logger.Printf("Stop all")
}

//HandlerClient some test worker for communication
func HandlerClient(cl fdstream.ClientSyncHander, instanceNum int) {
	defer wg.Done()
	var (
		i                 int
		totalSend         int
		err               error
		message, responce *fdstream.Message
	)
	for cl.IsAlive() {
		i++
		r := rand.Intn(500)
		time.Sleep(time.Duration(r) * time.Millisecond)
		name := fmt.Sprintf("Client%d-M%d", instanceNum, i)
		message = &fdstream.Message{
			Name:    []byte(name),
			Route:   []byte{},
			Payload: make([]byte, r*3, r*3),
		}
		totalSend += message.Len()
		if responce, err = cl.WriteAndReadResponce(message); err == nil {
			if !bytes.Equal(responce.Name, message.Name) {
				logger.Printf("Wrong responce client %d %dmessage %s want: %s", instanceNum, i, string(message.Name), string(responce.Name))
			}
		} else {
			logger.Printf("Error in responce client %d %dmessage %v", instanceNum, i, err)
		}
	}
	logger.Printf("Finish serving connection %d with total messages count: %d", instanceNum, i)
	logger.Printf("Finish serving connection %d with total bytes send: %d", instanceNum, totalSend)
}
