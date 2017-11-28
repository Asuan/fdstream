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
		//Exmaple handlers for connections
		//go HandleTCPWithoutRouting(conn, i)
		go HandlTCPWRouting(conn, i)
		i++
	}

	logger.Printf("Stop server server")

}

//HandleTCP handle income messages with routings to separate chans
func HandlTCPWRouting(conn *net.TCPConn, instanceNum int) error {
	defer conn.Close()
	logger.Printf("Handle connection: %s", conn.RemoteAddr().String())
	cl, err := fdstream.NewAsyncHandler(conn, conn)
	if err != nil {
		return err
	}

	//Create Router for client
	r, err := fdstream.NewRouterForClient(fmt.Sprintf("router%d", instanceNum), cl.(*fdstream.AsyncClient))
	if err != nil {
		return err
	}

	//Add routings and start router
	in1, out1 := make(chan *fdstream.Message, 100), make(chan *fdstream.Message, 100)
	r.AddRouting("0", in1, out1)
	in2, out2 := make(chan *fdstream.Message, 100), make(chan *fdstream.Message, 100)
	r.AddRouting("1", in2, out2)
	r.Start()
	var (
		i       int
		message *fdstream.Message
	)
	for cl.IsAlive() {
		i++
		backPaylod := []byte(fmt.Sprintf("Responce I-%d-#%d", instanceNum, i))
		select {
		case message = <-in1:
			logger.Printf("Get message router 1 message: %s", message.Name)
			out1 <- &fdstream.Message{
				Code:    0,
				Name:    message.Name,
				Payload: backPaylod,
			}
		case message = <-in2:
			logger.Printf("Get message router 2 message: %s", message.Name)
			out2 <- &fdstream.Message{
				Code:    0,
				Name:    message.Name,
				Payload: backPaylod,
			}
		}

		if err != nil {
			logger.Printf("Error respoonce: %v\n", err)
		}

	}

	logger.Printf("Finish serving connection %d with total messages count: %d", instanceNum, i)
	return nil
}

//HandleTCPWithoutRouting handle messages wihtout routings
func HandleTCPWithoutRouting(conn *net.TCPConn, instanceNum int) error {
	defer conn.Close()

	logger.Printf("Handle connection: %s", conn.RemoteAddr().String())
	cl, err := fdstream.NewAsyncHandler(conn, conn)
	if err != nil {
		return err
	}

	var (
		i int
	)
	for cl.IsAlive() {
		i++

		message := cl.Read()
		logger.Printf("Get message %s", message.Name)
		if err != nil {
			logger.Printf("Error respoonce: %v\n", err)
		}

		cl.Write(&fdstream.Message{
			Code:    0,
			Name:    message.Name,
			Payload: []byte(fmt.Sprintf("Responce I-%d-#%d", instanceNum, i)),
		})

	}

	logger.Printf("Finish serving connection %d with total messages count: %d", instanceNum, i)
	return nil
}
