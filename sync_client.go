package fdstream

import (
	"errors"
	"io"
	"sync"
	"time"
)

var (
	//ErrMessageTimeout indicate wait timeout appear
	ErrMessageTimeout = &Message{Code: erTimeoutCode, Name: "Timeout on waiting message"}
	//ErrMessageDuplicateName indicate sync client already wait message with same name
	ErrMessageDuplicateName = &Message{Code: erDuplicateNameErrorCode, Name: "Message with same name already wait response"}
)

type ClientSyncHander interface {
	AsyncWriter
	Read(string) (*Message, error)
	WriteAndReadResponce(*Message) (*Message, error)
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

//SyncClient single gorutine client for sync read write to Async client
type SyncClient struct {
	async *AsyncClient
	//Sync additional fields
	defaultTimeout time.Duration

	awaitMessageQ   chan *messageReciaver
	unknowMessage   map[string]*messageWithTimeout
	messageToReturn map[string]*messageReciaver
}

//NewSyncClient create sync handler it have sync read from stream
func NewSyncClient(outcome io.WriteCloser, income io.ReadCloser, timeout time.Duration) (ClientSyncHander, error) {
	a, _ := NewAsyncHandler(outcome, income)
	asyncClient := a.(*AsyncClient)
	c := &SyncClient{
		unknowMessage:   make(map[string]*messageWithTimeout, defaultQSize),
		messageToReturn: make(map[string]*messageReciaver, defaultQSize),
		awaitMessageQ:   make(chan *messageReciaver, defaultQSize),

		defaultTimeout: timeout,
		async:          asyncClient,
	}

	//Return specefied message
	returnWorker := func(c *SyncClient) {
		janitorTiker := time.NewTicker(c.defaultTimeout / 3)
		responceTiker := time.NewTicker(200 * time.Millisecond)
		defer janitorTiker.Stop()
		defer responceTiker.Stop()

		for asyncClient.IsAlive() {
			select {
			case <-janitorTiker.C: //cleanup old messages and responce waiters
				now := time.Now().UnixNano()
				for n, v := range c.unknowMessage {
					if v.timeout < now {
						delete(c.unknowMessage, n)
					}
				}
				for n, v := range c.messageToReturn { //fire timeout
					if v.timeout < now {
						v.responce <- ErrMessageTimeout
						delete(c.messageToReturn, n)
					}
				}

			case r := <-c.awaitMessageQ: //add message to wait responce from back side
				name := r.name
				if m, ok := c.unknowMessage[name]; ok {
					r.responce <- m.message
					delete(c.unknowMessage, name)
					continue
				}
				if _, ok := c.messageToReturn[name]; ok {
					r.responce <- ErrMessageDuplicateName
					continue
				}
				r.timeout = time.Now().Add(c.defaultTimeout).UnixNano()
				c.messageToReturn[name] = r

			case <-responceTiker.C: //Check messages and return it
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
				name := m.Name

				if r, ok := c.messageToReturn[name]; ok {
					r.responce <- m
					delete(c.messageToReturn, name)
					continue
				}
				c.unknowMessage[name] = &messageWithTimeout{
					message: m,
					timeout: time.Now().Add(c.defaultTimeout).UnixNano(),
				}
			}
		}

		for n, v := range c.messageToReturn { //TODO fire not timeout but another
			v.responce <- ErrMessageTimeout
			delete(c.messageToReturn, n)
		}
		for r := range c.awaitMessageQ {
			r.responce <- ErrMessageTimeout
		}

	}

	go returnWorker(c)
	return c, nil
}

//Write message to destination with async way
func (c *SyncClient) Write(m *Message) error {
	if m != nil {
		c.async.toSendMessageQ <- m
	}
	return errNilMessage
}

//WriteNamed write object to destination with async way
func (c *SyncClient) WriteNamed(code byte, name, route string, m Marshaller) (err error) {
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

//WriteBytes bytes to destination with async way
func (c *SyncClient) WriteBytes(code byte, name, route string, payload []byte) error {
	return c.Write(
		&Message{
			Code:    code,
			Name:    name,
			Route:   route,
			Payload: payload,
		})
}

//WriteAndReadResponce will write message and expect responce or error
func (c *SyncClient) WriteAndReadResponce(m *Message) (*Message, error) {
	if m == nil {
		return nil, errNilMessage
	}
	if len(m.Name) == 0 {
		return nil, ErrEmptyName
	}
	c.async.toSendMessageQ <- m
	return c.Read(m.Name)
}

//Read is wait and read message by specified name
func (c *SyncClient) Read(name string) (*Message, error) {
	getter := mrPool.Get().(*messageReciaver)
	getter.name = name
	c.awaitMessageQ <- getter
	mes := <-getter.responce
	mrPool.Put(getter)
	if mes.Code < 200 {
		return mes, nil
	}
	return nil, errors.New(mes.Name)
}

//IsAlive indicate is communication is alive or dead
func (c *SyncClient) IsAlive() bool {
	return c.async.IsAlive()
}
