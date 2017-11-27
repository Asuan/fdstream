package fdstream

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
)

const DefaultQSize = 200

var (
	ErrNilMessage = errors.New("Nil message")
)

type AsyncWriter interface {
	Write(*Message) error
	WriteNamed(byte, string, Marshaller) error
}

type Marshaller interface {
	Marshal() ([]byte, error)
}

type AsyncHandler interface {
	AsyncWriter
	IsAlive() bool
	Read() *Message
}

type AsyncClient struct {
	Destination    io.WriteCloser
	Source         io.ReadCloser
	toSendMessageQ chan *Message //Queue to send
	toReadMessageQ chan *Message //Queue to return back
	killer         sync.Once
	kill           chan bool
	alive          atomic.Value
}

//NewAsyncHandler create async handler
func NewAsyncHandler(outcome io.WriteCloser, income io.ReadCloser) (AsyncHandler, error) {
	c := &AsyncClient{
		Destination:    outcome,
		Source:         income,
		toSendMessageQ: make(chan *Message, DefaultQSize),
		toReadMessageQ: make(chan *Message, DefaultQSize),
		kill:           make(chan bool),
	}
	c.alive.Store(true)

	//Read message by message
	workerReader := func(c *AsyncClient) {
		var (
			err                    error
			n                      int
			lenN, lenR, lenB, lenP int
			cursor                 int
		)
		header := make([]byte, messageHeaderSize, messageHeaderSize)
		messageBody := make([]byte, MaxMessageSize, MaxMessageSize)
		for {
			n, err = c.Source.Read(header)
			if err != nil { //io.EOF or ect communication error
				c.shutdown()
				return
			}
			if n == 0 { //Skip empty read
				continue
			}

			if n != messageHeaderSize { //Wrong message header read in broken state
				c.shutdown()
				return
			}
			//TODO optimize reading
			m := newMessage(UnmarshalHeader(header))
			messageBody = messageBody[:m.Len()-messageHeaderSize]
			lenN, lenR, lenP = len(m.Name), len(m.Route), len(m.Payload)
			if lenB, err = c.Source.Read(messageBody); lenB == len(messageBody) && err == nil {
				cursor = lenN
				if lenN > 0 {
					copy(m.Name, messageBody[0:cursor])
				}
				if lenR > 0 {
					copy(m.Route, messageBody[cursor:cursor+lenR])
					cursor += lenR
				}
				if lenP > 0 {
					copy(m.Payload, messageBody[cursor:cursor+lenP])
				}
			} else {
				c.shutdown()
				return
			}
			c.toReadMessageQ <- m
		}
	}

	//Write message by message
	workerWriter := func(c *AsyncClient) {
		for {
			select {
			case <-c.kill:
				return
			case m := <-c.toSendMessageQ:
				if bytes, err := m.Marshal(); err == nil {
					if _, err := c.Destination.Write(bytes); err != nil {
						c.shutdown()
						return
					}
				}
			}

		}
	}

	go workerReader(c)
	go workerWriter(c)
	return c, nil
}

//Write will write message to destination
func (c *AsyncClient) Write(m *Message) error {
	if m != nil {
		c.toSendMessageQ <- m
		return nil
	}
	return ErrNilMessage
}

func (c *AsyncClient) WriteNamed(code byte, name string, m Marshaller) (err error) {
	var b []byte
	if b, err = m.Marshal(); err == nil {
		return c.Write(
			&Message{
				Code:    code,
				Name:    []byte(name), //TODO unsafe byte read
				Payload: b,
			})
	}
	return err

}

func (c *AsyncClient) Read() *Message {
	return <-c.toReadMessageQ
}

//Shutdown close all read and write strams but save unreaded or unwrited data.
func (c *AsyncClient) shutdown() {
	//DO not close chans need grace safe inprogress messages
	c.killer.Do(func() {
		c.Source.Close()      //It will stop read worker
		c.Destination.Close() //It will fire error on writer writing message
		close(c.kill)         //It should stop writer
		//TODO save unprocessed data
		//TODO kill instance to clean data in q

	})
	c.alive.Store(false)
}

func (c *AsyncClient) IsAlive() bool {
	return c.alive.Load().(bool)
}
