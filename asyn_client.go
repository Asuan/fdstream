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
	WriteNamed(byte, string, string, Marshaller) error
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
	OutputStream   io.WriteCloser
	InputStream    io.ReadCloser
	toSendMessageQ chan *Message //Queue to send
	toReadMessageQ chan *Message //Queue to return back
	killer         sync.Once
	kill           chan bool
	alive          atomic.Value
}

//NewAsyncHandler create async handler
func NewAsyncHandler(outcome io.WriteCloser, income io.ReadCloser) (AsyncHandler, error) {
	c := &AsyncClient{
		OutputStream:   outcome,
		InputStream:    income,
		toSendMessageQ: make(chan *Message, DefaultQSize),
		toReadMessageQ: make(chan *Message, DefaultQSize),
		kill:           make(chan bool),
	}
	c.alive.Store(true)

	//Read message by message
	workerReader := func(c *AsyncClient, toReadMessage chan<- *Message) {
		var (
			err        error
			n          int
			lenB       int //full length of body without header
			lenR, lenP uint16
			cursor     uint16
			code       byte
			eof        bool
		)
		header := make([]byte, messageHeaderSize, messageHeaderSize)
		messageBody := make([]byte, MaxMessageSize, MaxMessageSize)
		for !eof {
			n, err = c.InputStream.Read(header)
			if err != nil {
				break
			}
			if n == 0 { //Skip empty read
				continue
			}

			if n != messageHeaderSize { //Wrong message header read in broken state
				break
			}
			//TODO optimize reading
			code, cursor, lenR, lenP = UnmarshalHeader(header)
			m := &Message{
				Code:    code,
				Payload: make([]byte, lenP, lenP),
			}
			messageBody = messageBody[:(cursor + lenR + lenP)]

			lenB, err = c.InputStream.Read(messageBody)
			if err != nil { //try read message it EOF appear
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

			toReadMessage <- m
		}
		c.shutdown()
	}

	//Write message by message
	workerWriter := func(c *AsyncClient, toSendMessages <-chan *Message) {
		for {
			select {
			case <-c.kill:
				return
			case m := <-c.toSendMessageQ:
				if bytes, err := m.Marshal(); err == nil {
					if _, err := c.OutputStream.Write(bytes); err != nil {
						c.shutdown()
						return
					}
				}
			}

		}
	}

	go workerReader(c, c.toReadMessageQ)
	go workerWriter(c, c.toSendMessageQ)
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

func (c *AsyncClient) Read() *Message {
	return <-c.toReadMessageQ
}

//Shutdown close all read and write strams but save unreaded or unwrited data.
func (c *AsyncClient) shutdown() {
	//DO not close chans need grace safe inprogress messages
	c.killer.Do(func() {
		c.InputStream.Close()  //It will stop read worker
		c.OutputStream.Close() //It will fire error on writer writing message
		close(c.kill)          //It should stop writer
		//TODO save unprocessed data
		//TODO kill instance to clean data in q

	})
	c.alive.Store(false)
}

func (c *AsyncClient) IsAlive() bool {
	return c.alive.Load().(bool)
}
