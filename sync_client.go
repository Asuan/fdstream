package fdstream

import (
	"errors"
	"io"
	"sync"
	"time"
)

var (
	ErrTimeout       = errors.New("Timeout on waiting message")
	ErrMessageMarker = &Message{Code: 255}
)

type ClientSyncReader interface {
	Read(string) (*Message, error)
	WriteAndReadResponce(*Message) (*Message, error)
}

type ClientSyncHander interface {
	AsyncWriter
	ClientSyncReader
	IsAlive() bool
}

type messageWithTimeout struct {
	message *Message
	timeout int64
}

type messageReciaver struct {
	responce chan *Message
	name     string
	timeout  int64
}

var mrPool = sync.Pool{
	New: func() interface{} {
		return &messageReciaver{responce: make(chan *Message, 1)}
	},
}

//SyncronizedSingleToneClient single gorutine client for sync read write to Async client
type SyncronizedSingleToneClient struct {
	async *AsyncClient
	//Sync additional fields
	defaultTimeout time.Duration

	returnerMessageQ chan *messageReciaver
	unknowMessage    map[string]*messageWithTimeout
	messageToReturn  map[string]*messageReciaver
}

//NewTCPSyncClientHandler create sync handler it have sync read from stream
func NewSyncClient(outcome io.WriteCloser, income io.ReadCloser, timeout time.Duration) (ClientSyncHander, error) {
	a, _ := NewAsyncHandler(outcome, income)
	asyncClient := a.(*AsyncClient)
	c := &SyncronizedSingleToneClient{
		unknowMessage:    make(map[string]*messageWithTimeout, DefaultQSize),
		messageToReturn:  make(map[string]*messageReciaver, DefaultQSize),
		returnerMessageQ: make(chan *messageReciaver, DefaultQSize),

		defaultTimeout: timeout,
		async:          asyncClient,
	}

	//Return specefied message
	returnWorker := func(c *SyncronizedSingleToneClient) {
		tiker := time.NewTicker(c.defaultTimeout / 3)
		returnTiker := time.NewTicker(200 * time.Millisecond)
		defer tiker.Stop()
		defer returnTiker.Stop()

		for asyncClient.alive {
			select {
			case <-tiker.C: //cleanup old messages
				now := time.Now().UnixNano()
				for n, v := range c.unknowMessage {
					if v.timeout < now {
						delete(c.unknowMessage, n)
					}
				}
				for n, v := range c.messageToReturn { //fire timeout
					if v.timeout < now {
						v.responce <- ErrMessageMarker
						delete(c.messageToReturn, n)
					}
				}

			case r := <-c.returnerMessageQ: //add message to wait responce from back side
				name := r.name
				if m, ok := c.unknowMessage[name]; ok {
					r.responce <- m.message
					delete(c.unknowMessage, name)
				} else {
					r.timeout = time.Now().Add(c.defaultTimeout).UnixNano()
					c.messageToReturn[name] = r
				}

			case <-returnTiker.C: //Return responce messages
				for k, v := range c.messageToReturn {
					if m, ok := c.unknowMessage[k]; ok {
						v.responce <- m.message
						delete(c.unknowMessage, k)
						delete(c.messageToReturn, k)
					}
				}
				if !c.IsAlive() {
					return
				}
			case m := <-asyncClient.toReadMessageQ: //read income messages
				name := string(m.Name)
				if r, ok := c.messageToReturn[name]; ok {
					r.responce <- m
					delete(c.messageToReturn, name)
				} else {
					c.unknowMessage[string(m.Name)] = &messageWithTimeout{
						message: m,
						timeout: time.Now().Add(c.defaultTimeout).UnixNano(),
					}
				}
			}
		}

		for n, v := range c.messageToReturn { //TODO fire not timeout but another
			v.responce <- ErrMessageMarker
			delete(c.messageToReturn, n)
		}
		for r := range c.returnerMessageQ {
			r.responce <- ErrMessageMarker
		}

	}

	go returnWorker(c)
	return c, nil
}

//Write asyncly message to destination
func (c *SyncronizedSingleToneClient) Write(m *Message) error {
	if m != nil {
		c.async.toSendMessageQ <- m
	}
	return ErrNilMessage
}

//WriteNamed asyncly message to destination
func (c *SyncronizedSingleToneClient) WriteNamed(code byte, name string, m Marshaller) error {
	if b, err := m.Marshal(); err == nil {
		return c.Write(
			&Message{
				Code:    code,
				Name:    []byte(name),
				Payload: b,
			})
	} else {
		return err
	}

}

//WriteAndReadResponce will write message and expect responce or error
func (c *SyncronizedSingleToneClient) WriteAndReadResponce(m *Message) (*Message, error) {
	if m == nil {
		return nil, ErrNilMessage
	}
	if len(m.Name) == 0 {
		return nil, ErrEmptyName
	}
	c.async.toSendMessageQ <- m
	return c.Read(string(m.Name))
}

//Read read message by specified name
func (c *SyncronizedSingleToneClient) Read(name string) (*Message, error) {
	getter := mrPool.Get().(*messageReciaver)
	getter.name = name
	c.returnerMessageQ <- getter
	mes := <-getter.responce
	mrPool.Put(getter)
	if mes.Code != 255 {
		return mes, nil
	}

	return nil, ErrTimeout

}

func (c *SyncronizedSingleToneClient) IsAlive() bool {
	return c.async.alive
}
