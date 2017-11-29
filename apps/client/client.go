package main

import (
	"flag"
	"fmt"
	"runtime/pprof"
	"strconv"

	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Asuan/fdstream"
)

type context struct {
	isSync  bool
	server  string
	tcpAddr *net.TCPAddr
}

var (
	ctx           context
	wg            sync.WaitGroup
	logger        *log.Logger
	cpuprofile    string
	totalBytes    *int64
	totalMessages *int64
	totalWait     *int64
)

//Initialize flags
func Initialize() {
	var (
		err error
	)
	var a, b, c int64
	totalBytes = &a
	totalMessages = &b
	totalWait = &c
	flag.BoolVar(&ctx.isSync, "sync", true, "mode of client")
	flag.StringVar(&ctx.server, "server", "0.0.0.0:1900", "address of server")
	//Profile
	flag.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile `file`")

	flag.Parse()

	if ctx.tcpAddr, err = net.ResolveTCPAddr("tcp", ctx.server); err != nil {
		fmt.Println("Wrong address: " + err.Error())
	}

	//	logger = log.New(ioutil.Discard, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
	logger = log.New(os.Stdout, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile)
}

func main() {
	Initialize()

	//Optional profiling
	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()
	}
	timeStart := time.Now()
	conn, err := net.DialTCP("tcp", nil, ctx.tcpAddr)
	if err != nil {
		logger.Printf("Could not connect to address: %s error: %v", ctx.tcpAddr.String(), err)
	}
	defer conn.Close()
	//Tune tcp connection
	conn.SetReadBuffer(fdstream.MaxMessageSize * 40)
	conn.SetWriteBuffer(fdstream.MaxMessageSize * 40)
	conn.SetKeepAlive(true)

	//Create new communication client with timeout for messages inside stream 2 second
	cl, err := fdstream.NewSyncClient(conn, conn, time.Duration(2*time.Minute))
	if err != nil {
		logger.Printf("Could not create instance %v", err)
	}
	//Create some async client for communication with server
	wg.Add(200)
	for i := 0; i < 200; i++ {
		logger.Printf("Start client %d", i)
		go HandlerClient(cl, i)
	}
	wg.Wait()

	//Some end statistic of communication
	duration := time.Now().Sub(timeStart)
	logger.Printf("Data rate: %f", float64(*totalBytes)/duration.Seconds())
	logger.Printf("Hits: %f", float64(*totalMessages)/duration.Seconds())
	logger.Printf("Average wait: %v", time.Duration(*totalWait / *totalMessages))
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
		r := rand.Intn(100)
		time.Sleep(time.Duration(r) * time.Millisecond)

		//create message with unique name for sending data and waiting responce by specified name
		message = &fdstream.Message{
			Name:    fmt.Sprintf("Client%d-M%d", instanceNum, i),
			Route:   strconv.Itoa(i % 2), //Simple route rounding
			Payload: make([]byte, r*15, r*15),
		}
		totalSend += message.Len()
		start := time.Now()
		if responce, err = cl.WriteAndReadResponce(message); err == nil {
			if responce.Name != message.Name {
				logger.Printf("Wrong responce client %d %dmessage %s want: %s", instanceNum, i, string(message.Name), string(responce.Name))
			}
		} else {
			logger.Printf("Error in responce client %d %dmessage %v", instanceNum, i, err)
		}
		delta := time.Now().Sub(start)
		logger.Printf("Wait responce during %v client %d %dmessage", delta, instanceNum, i)
		atomic.AddInt64(totalWait, int64(delta))
	}
	logger.Printf("Finish serving connection %d with total messages count: %d", instanceNum, i)
	logger.Printf("Finish serving connection %d with total bytes send: %d", instanceNum, totalSend)
	atomic.AddInt64(totalBytes, int64(totalSend))
	atomic.AddInt64(totalMessages, int64(i))

}
