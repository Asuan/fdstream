package fdstream

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"
)

//TODO support longer mesages

//Max size of message
const (
	MaxMessageSize = 1e5
)

var (
	ErrEmptyName       = errors.New("Empty name of message")
	ErrTooShortMessage = errors.New("Too short message")
	ErrBinaryLength    = errors.New("Incorrect binary length")
)

type Message struct {
	Code    byte
	Name    []byte
	Route   []byte
	Payload []byte
}

var bufferPool = sync.Pool{}

func getBuf() *bytes.Buffer {
	if b := bufferPool.Get(); b != nil {
		buf := b.(*bytes.Buffer)
		buf.Reset()
		return buf
	}
	return bytes.NewBuffer(make([]byte, 0, MaxMessageSize))

}

//Marshal marshall message to byte array with simple structure [code,name length, value length, name,value]
func (m *Message) Marshal() ([]byte, error) {
	buf := getBuf()

	buf.WriteByte(m.Code)
	uintNamelen := uint16(len(m.Name))
	uintValueLen := uint16(len(m.Payload))
	uintRouteLen := uint16(len(m.Route))
	binary.Write(buf, binary.BigEndian, uintNamelen)
	binary.Write(buf, binary.BigEndian, uintRouteLen)
	binary.Write(buf, binary.BigEndian, uintValueLen)

	buf.Write(m.Name)
	buf.Write(m.Route)
	buf.Write(m.Payload)
	res := buf.Bytes()
	bufferPool.Put(buf)
	return res, nil
}

//Unmarshal create message from specified byte array
func Unmarshal(b []byte) (*Message, error) {

	if len(b) < 7 {
		return nil, ErrTooShortMessage
	}
	var (
		code                          byte
		m                             *Message
		nameLen, routeLen, payloadLen uint16
	)

	code, nameLen, routeLen, payloadLen = UnmarshalHeader(b)
	m = newMessage(code, nameLen, routeLen, payloadLen)
	if len(b) != int(7+nameLen+routeLen+payloadLen) {
		return nil, ErrBinaryLength
	}

	//TODO optimize
	if nameLen > 0 {
		copy(m.Name, b[7:7+int(nameLen)])
	}
	if routeLen > 0 {
		copy(m.Route, b[7+int(nameLen):7+int(nameLen)+int(routeLen)])
	}

	if payloadLen > 0 {
		copy(m.Payload, b[7+int(nameLen)+int(routeLen):7+int(nameLen)+int(routeLen)+int(payloadLen)])
	}
	return m, nil

}

//UnmarshalHeader is unsafe read expect at least 5 bytes length
func UnmarshalHeader(b []byte) (code byte, nameLen, routeLen, payloadLen uint16) {
	code = b[0]
	nameLen = binary.BigEndian.Uint16(b[1:3])
	routeLen = binary.BigEndian.Uint16(b[3:5])
	payloadLen = binary.BigEndian.Uint16(b[5:7])
	return
}

func newMessage(acion byte, nameLen, routeLen, payloadLen uint16) *Message {
	return &Message{
		Code:    acion,
		Name:    make([]byte, int(nameLen), int(nameLen)),
		Route:   make([]byte, int(routeLen), int(routeLen)),
		Payload: make([]byte, int(payloadLen), int(payloadLen)),
	}
}

func (m *Message) Len() int {
	return 7 + len(m.Name) + len(m.Route) + len(m.Payload)
}