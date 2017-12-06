package fdstream

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

var (
	//ErrMessageTimeout indicate wait timeout appear
	ErrMessageTimeout = &Message{Code: erTimeoutCode, Name: "Timeout on waiting message"}
	//ErrMessageDuplicateID indicate sync client already wait message with same name
	ErrMessageDuplicateID = &Message{Code: erDuplicateIdErrorCode, Name: "Message with same id already wait response"}
)

type ClientSyncHander interface {
	AsyncWriter
	WriteAndReadResponce(*Message) (*Message, error)
	IsAlive() bool
}

type messageWithTimeout struct {
	message *Message
	timeout int64
}

type messageReceiver struct {
	responce chan *Message
	id       uint32
	timeout  int64
}

var (
	messageReceiverPool = sync.Pool{
		New: func() interface{} {
			return &messageReceiver{responce: make(chan *Message, 1)}
		},
	}
	messageWaiterPool = sync.Pool{
		New: func() interface{} {
			return &messageWithTimeout{}
		},
	}
)

//SyncClient single goroutine client for sync read write to Async client
type SyncClient struct {
	async *AsyncClient
	//Sync additional fields
	defaultTimeout  time.Duration
	counter         *uint32
	awaitMessageQ   chan *messageReceiver
	unknownMessage  map[uint32]*messageWithTimeout
	messageToReturn map[uint32]*messageReceiver
}

//NewSyncClient create sync handler it have sync read from stream
func NewSyncClient(outcome io.WriteCloser, income io.ReadCloser, timeout time.Duration) (ClientSyncHander, error) {
	a, _ := NewAsyncHandler(outcome, income)
	var z uint32
	asyncClient := a.(*AsyncClient)
	c := &SyncClient{
		unknownMessage:  make(map[uint32]*messageWithTimeout, defaultQSize),
		messageToReturn: make(map[uint32]*messageReceiver, 5*defaultQSize),
		awaitMessageQ:   make(chan *messageReceiver, defaultQSize),
		counter:         &z,
		defaultTimeout:  timeout,
		async:           asyncClient,
	}

	//Return specified message
	returnWorker := func(sync *SyncClient) {
		var (
			id  uint32
			ok  bool
			now int64
			mwt *messageWithTimeout
			mr  *messageReceiver
		)
		janitorTicker := time.NewTicker(sync.defaultTimeout / 3)
		defer janitorTicker.Stop()

		for asyncClient.IsAlive() {
			select {
			case <-janitorTicker.C: //cleanup old messages and responce waiters
				now = time.Now().UnixNano()
				for id, mwt = range sync.unknownMessage {
					if mwt.timeout < now {
						delete(sync.unknownMessage, id)
					}
				}
				for id, mr = range sync.messageToReturn { //fire timeout
					if mr.timeout < now {
						mr.responce <- ErrMessageTimeout
						delete(sync.messageToReturn, id)
					}
				}

			case r := <-sync.awaitMessageQ: //add messageReceiver to wait responce from back side
				id = r.id
				if mwt, ok = sync.unknownMessage[id]; ok {
					r.responce <- mwt.message
					messageWaiterPool.Put(mwt)
					delete(sync.unknownMessage, id)
					continue
				}
				if _, ok = sync.messageToReturn[id]; ok { //TODO maybe just remove check
					r.responce <- ErrMessageDuplicateID
					continue
				}
				r.timeout = time.Now().Add(sync.defaultTimeout).UnixNano()
				sync.messageToReturn[id] = r
			case m := <-asyncClient.toReadMessageQ: //read income messages
				id = m.Id

				if mr, ok = sync.messageToReturn[id]; ok {
					mr.responce <- m
					delete(sync.messageToReturn, id)
					continue
				}
				waitMessage := messageWaiterPool.Get().(*messageWithTimeout)
				waitMessage.message = m
				waitMessage.timeout = time.Now().Add(sync.defaultTimeout).UnixNano()
				sync.unknownMessage[id] = waitMessage
			}
		}

		for n, v := range sync.messageToReturn { //TODO fire not timeout but another
			v.responce <- ErrMessageTimeout
			delete(sync.messageToReturn, n)
		}
		for r := range sync.awaitMessageQ {
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
func (c *SyncClient) WriteNamed(code byte, name string, m Marshaler) (err error) {
	var b []byte
	if b, err = m.Marshal(); err == nil {
		return c.Write(NewMessage(code, name, b))
	}
	return err
}

//WriteBytes bytes to destination with async way
func (c *SyncClient) WriteBytes(code byte, name string, payload []byte) error {
	return c.Write(NewMessage(code, name, payload))
}

//WriteAndReadResponce will write message and expect responce or error
func (c *SyncClient) WriteAndReadResponce(m *Message) (*Message, error) {
	if m == nil {
		return nil, errNilMessage
	}

	m.Id = atomic.AddUint32(c.counter, 1)
	if len(m.Name) == 0 {
		return nil, ErrEmptyName
	}
	c.async.toSendMessageQ <- m
	return c.read(m.Id)
}

//read is wait and read message by specified id
func (c *SyncClient) read(id uint32) (*Message, error) {
	getter := messageReceiverPool.Get().(*messageReceiver)
	getter.id = id
	c.awaitMessageQ <- getter
	mes := <-getter.responce
	messageReceiverPool.Put(getter)
	if mes.Code < 200 {
		return mes, nil
	}
	return nil, errors.New(mes.Name)
}

//IsAlive indicate is communication is alive or dead
func (c *SyncClient) IsAlive() bool {
	return c.async.IsAlive()
}
