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
	ErrMessageDuplicateID = &Message{Code: erDuplicateIDErrorCode, Name: "Message with same id already wait response"}
)

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
	*AsyncClient
	//Sync additional fields
	defaultTimeout  time.Duration
	counter         *uint32
	awaitMessageQ   chan *messageReceiver
	unknownMessage  map[uint32]*messageWithTimeout
	messageToReturn map[uint32]*messageReceiver
}

//NewSyncClient create sync handler it have sync read from stream
func NewSyncClient(outcome io.WriteCloser, income io.ReadCloser, timeout time.Duration) (*SyncClient, error) {
	asyncClient, err := NewAsyncClient(outcome, income)
	if err != nil {
		return nil, err
	}

	c := &SyncClient{
		unknownMessage:  make(map[uint32]*messageWithTimeout, 10*defaultQSize),
		messageToReturn: make(map[uint32]*messageReceiver, 20*defaultQSize),
		awaitMessageQ:   make(chan *messageReceiver, defaultQSize),
		counter:         new(uint32),
		defaultTimeout:  timeout,
		AsyncClient:     asyncClient,
	}

	go c.synchronizationWorker()
	return c, nil
}

func (sync *SyncClient) synchronizationWorker() {
	var (
		id          uint32
		ok          bool
		now         int64
		mwt         *messageWithTimeout
		mr          *messageReceiver
		asyncClient = sync.AsyncClient
	)
	janitorTicker := time.NewTicker(sync.defaultTimeout / 3)
	defer janitorTicker.Stop()

	for asyncClient.IsAlive() {
		select {
		case <-janitorTicker.C: //cleanup old messages and responce waiters
			now = time.Now().UnixNano()
			for id, mwt = range sync.unknownMessage {
				if mwt.timeout > now {
					continue
				}
				delete(sync.unknownMessage, id)
			}
			for id, mr = range sync.messageToReturn { //fire timeout
				if mr.timeout > now {
					continue
				}
				mr.responce <- ErrMessageTimeout
				delete(sync.messageToReturn, id)
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
		case m := <-asyncClient.ToReadQ: //read income messages
			id = m.ID

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
	// fail rest messages
	for id, mr = range sync.messageToReturn { //fire timeout
		mr.responce <- ErrMessageTimeout
	}
	for mr = range sync.awaitMessageQ {
		mr.responce <- ErrMessageTimeout
	}
}

//WriteNamed write object to destination with async way
func (sync *SyncClient) WriteNamed(code byte, name string, m Marshaler) (err error) {
	var b []byte
	if b, err = m.Marshal(); err == nil {
		sync.Write(NewMessage(code, name, b))
		return nil
	}
	return err
}

//WriteBytes bytes to destination with async way
func (sync *SyncClient) WriteBytes(code byte, name string, payload []byte) {
	sync.Write(NewMessage(code, name, payload))
}

//WriteAndReadResponce will write message and expect responce or error
func (sync *SyncClient) WriteAndReadResponce(m *Message) (*Message, error) {
	if m == nil {
		return nil, errNilMessage
	}

	m.ID = atomic.AddUint32(sync.counter, 1)
	if len(m.Name) == 0 {
		return nil, ErrEmptyName
	}
	sync.AsyncClient.ToSendQ <- m
	return sync.read(m.ID)
}

//read is 'wait and read' message by specified id
func (sync *SyncClient) read(id uint32) (*Message, error) {
	getter := messageReceiverPool.Get().(*messageReceiver)
	getter.id = id
	sync.awaitMessageQ <- getter
	mes := <-getter.responce
	messageReceiverPool.Put(getter)
	if mes.Code < 200 {
		return mes, nil
	}
	return nil, errors.New(mes.Name)
}
