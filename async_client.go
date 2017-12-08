package fdstream

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

const defaultQSize = 200

var (
	errNilMessage = errors.New("Nil message")
)

//Marshaller interface to pass custom object it is same with many *Marshal interfaces
type Marshaler interface {
	Marshal() ([]byte, error)
}

type AsyncClient struct {
	OutputStream   io.Writer
	InputStream    io.ReadCloser
	toSendMessageQ chan *Message //Queue to send to remote
	toReadMessageQ chan *Message //Queue to read message
	killer         *sync.Once
	restorer       *sync.Once
	kill           chan bool
	alive          atomic.Value
}

//NewAsyncHandler create async handler
func NewAsyncHandler(outcome io.Writer, income io.ReadCloser) (*AsyncClient, error) {
	c := &AsyncClient{
		OutputStream:   outcome,
		InputStream:    income,
		toSendMessageQ: make(chan *Message, defaultQSize),
		toReadMessageQ: make(chan *Message, defaultQSize),
		kill:           make(chan bool, 1),
		killer:         new(sync.Once),
		restorer:       new(sync.Once),
	}
	c.alive.Store(true)

	go c.workerReader(c.toReadMessageQ)
	go c.workerWriter(c.toSendMessageQ)
	return c, nil
}

//Read message by message from input reader
func (c *AsyncClient) workerReader(outcome chan<- *Message) {
	var (
		err         error
		n           int
		lenB        int //full length of body without header
		lenP        uint16
		cursor      uint16
		id          uint32
		code        byte
		eof         bool
		header      = make([]byte, messageHeaderSize, messageHeaderSize)
		messageBody = make([]byte, MaxMessageSize, MaxMessageSize)
	)

	for !eof {
		n, err = c.InputStream.Read(header)
		if err != nil {
			break //If we get error so looks like no way to continue
		}
		if n == 0 { //Skip empty read
			continue
		}

		if n != messageHeaderSize { //Wrong message header read looks broken state
			break
		}
		//TODO optimize reading
		code, id, cursor, lenP = unmarshalHeader(header)
		m := &Message{
			Code: code,
			Id:   id,

			//This is so slow operation but we should copy data from messageBody buffer
			Payload: make([]byte, lenP, lenP),
		}
		messageBody = messageBody[:(cursor + lenP)]

		lenB, err = c.InputStream.Read(messageBody)
		if err != nil {
			//try read last message if EOF appear
			if err != io.EOF || lenB != len(messageBody) {
				break
			}
			eof = true
		}

		if cursor > 0 {
			m.Name = string(messageBody[0:cursor])
		}

		if lenP > 0 {
			copy(m.Payload, messageBody[cursor:cursor+lenP])
		}

		outcome <- m
	}
	c.Shutdown()
}

//Write message by message to output reader from chan it can be run in multiple instances
func (c *AsyncClient) workerWriter(income <-chan *Message) {
	var (
		err error
		m   *Message
	)
mainLoop:
	for {
		select {
		case <-c.kill:
			break mainLoop
		case m = <-income:
			if _, err = m.WriteTo(c.OutputStream); err != nil {
				break mainLoop
			}
		}
	}
	c.Shutdown()
}

//Write will write message to destination
//The function is thread safe
func (c *AsyncClient) Write(m *Message) {
	c.toSendMessageQ <- m
}

//WriteNamed will write marshalable object to destination
//The function is thread safe
func (c *AsyncClient) WriteNamed(code byte, name string, m Marshaler) (err error) {
	var b []byte
	if b, err = m.Marshal(); err == nil {
		c.Write(NewMessage(code, name, b))
		return nil
	}
	return err

}

//WriteBytes will write bytes to destination
//The function is thread safe
func (c *AsyncClient) WriteBytes(code byte, name string, payload []byte) {
	c.Write(NewMessage(code, name, payload))

}

//Read message read message from internal chan
//The function is thread safe
func (c *AsyncClient) Read() *Message {
	return <-c.toReadMessageQ
}

//Shutdown close  read but save un-readed or un-writhed data.
func (c *AsyncClient) Shutdown() {
	//DO not close chans need grace safe in-progress messages
	c.killer.Do(func() {
		close(c.kill)         //It should stop writer
		c.InputStream.Close() //We should notify all 3d writes about trouble.

		c.restorer = new(sync.Once) //To keep way to restore
	})
	c.alive.Store(false)
}

//IsAlive notify about state of async client
func (c *AsyncClient) IsAlive() bool {
	return c.alive.Load().(bool)
}

//Restore restore after client after killing
func (c *AsyncClient) Restore(outcome io.Writer, income io.ReadCloser) {
	if !c.IsAlive() {
		c.restorer.Do(func() {
			c.killer = new(sync.Once)
			c.InputStream = income
			c.OutputStream = outcome
			c.kill = make(chan bool, 1)
			c.alive.Store(true)
			go c.workerReader(c.toReadMessageQ)
			go c.workerWriter(c.toSendMessageQ)

		})
	}
}
