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

type AsyncWriter interface {
	Write(*Message) error
	WriteNamed(byte, string, string, Marshaller) error
	WriteBytes(byte, string, string, []byte) error
}

//Marshaller interface to pass custom object it is same with many *Marshal interfaces
type Marshaller interface {
	Marshal() ([]byte, error)
}

type AsyncHandler interface {
	AsyncWriter
	IsAlive() bool
	Read() *Message
}

type AsyncClient struct {
	OutputStream   io.Writer
	InputStream    io.ReadCloser
	toSendMessageQ chan *Message //Queue to send to remote
	toReadMessageQ chan *Message //Queue to to read message
	killer         sync.Once
	kill           chan bool
	alive          atomic.Value
}

//NewAsyncHandler create async handler
func NewAsyncHandler(outcome io.Writer, income io.ReadCloser) (AsyncHandler, error) {
	c := &AsyncClient{
		OutputStream:   outcome,
		InputStream:    income,
		toSendMessageQ: make(chan *Message, defaultQSize),
		toReadMessageQ: make(chan *Message, defaultQSize),
		kill:           make(chan bool),
	}
	c.alive.Store(true)

	//Read message by message from input reader
	workerReader := func(c *AsyncClient, outcome chan<- *Message) {
		var (
			err         error
			n           int
			lenB        int //full length of body without header
			lenR, lenP  uint16
			cursor      uint16
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
			//TODO optimize reading and use default unmarshal func
			code, cursor, lenR, lenP = UnmarshalHeader(header)
			m := &Message{
				Code:    code,
				Payload: make([]byte, lenP, lenP),
			}
			messageBody = messageBody[:(cursor + lenR + lenP)]

			lenB, err = c.InputStream.Read(messageBody)
			if err != nil {
				//try read last message it EOF appear
				if err != io.EOF || lenB != len(messageBody) {
					break
				}
				eof = true
			}

			if cursor > 0 {
				m.Name = string(messageBody[0:cursor])
			}
			if lenR > 0 {
				m.Route = string(messageBody[cursor : cursor+lenR])
				cursor += lenR
			}
			if lenP > 0 {
				copy(m.Payload, messageBody[cursor:cursor+lenP])
			}

			outcome <- m
		}
		c.shutdown()
	}

	//Write message by message to outut reader from chan
	workerWriter := func(c *AsyncClient, income <-chan *Message) {
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
		c.shutdown()
	}

	go workerReader(c, c.toReadMessageQ)
	go workerWriter(c, c.toSendMessageQ)
	return c, nil
}

//Write will write message to destination
//The function is thread safe
func (c *AsyncClient) Write(m *Message) error {
	if m != nil {
		c.toSendMessageQ <- m
		return nil
	}
	return errNilMessage
}

//WriteNamed will write marshalable object to destination
//The function is thread safe
func (c *AsyncClient) WriteNamed(code byte, name, route string, m Marshaller) (err error) {
	var b []byte
	if b, err = m.Marshal(); err == nil {
		return c.Write(
			&Message{
				Code:    code,
				Name:    name,
				Route:   route,
				Payload: b,
			})
	}
	return err

}

//WriteBytes will write bytes to destination
//The function is thread safe
func (c *AsyncClient) WriteBytes(code byte, name, route string, payload []byte) (err error) {
	return c.Write(
		&Message{
			Code:    code,
			Name:    name,
			Route:   route,
			Payload: payload,
		})
}

//Read message read message from internal chan
//The function is thread safe
func (c *AsyncClient) Read() *Message {
	return <-c.toReadMessageQ
}

//Shutdown close all read and write strams but save unreaded or unwrited data.
func (c *AsyncClient) shutdown() {
	//DO not close chans need grace safe inprogress messages
	c.killer.Do(func() {
		close(c.kill)         //It should stop writer
		c.InputStream.Close() //We should notify all 3d writes about trouble.
		//TODO save unprocessed data
		//TODO kill instance to clean data in q

	})
	c.alive.Store(false)
}

//IsAlive notify about state of async client
func (c *AsyncClient) IsAlive() bool {
	return c.alive.Load().(bool)
}
