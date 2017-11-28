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
	MaxMessageSize          = 1e5
	messageHeaderSize       = 7
	erMissRoutingCode  byte = 254
	erGeneralErrorCode byte = 255
	erTimeoutCode      byte = 253
)

var (
	ErrEmptyName       = errors.New("Empty name of message")
	ErrTooShortMessage = errors.New("Too short message")
	ErrBinaryLength    = errors.New("Incorrect binary length")
)

type Message struct {
	Name  string
	Route string
	//Code is an a flag of somebody, fill free to use flag < 200
	//code with value more than 200 mean some error or problem
	Code    byte
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
	b := make([]byte, 2, 2)
	binary.BigEndian.PutUint16(b, uintNamelen)
	buf.Write(b)
	binary.BigEndian.PutUint16(b, uintRouteLen)
	buf.Write(b)
	binary.BigEndian.PutUint16(b, uintValueLen)
	buf.Write(b)

	buf.WriteString(m.Name)
	buf.WriteString(m.Route)
	buf.Write(m.Payload)
	res := buf.Bytes()
	bufferPool.Put(buf)
	return res, nil
}

//Unmarshal create message from specified byte array
func Unmarshal(b []byte) (*Message, error) {

	if len(b) < messageHeaderSize {
		return nil, ErrTooShortMessage
	}
	var (
		code                         byte
		m                            *Message
		cursor, routeLen, payloadLen uint16
	)

	code, cursor, routeLen, payloadLen = UnmarshalHeader(b)
	m = &Message{
		Code: code,
	}
	if len(b) != int(messageHeaderSize+cursor+routeLen+payloadLen) {
		return nil, ErrBinaryLength
	}

	//TODO optimize
	if cursor > 0 {
		m.Name = string(b[messageHeaderSize : messageHeaderSize+cursor])
	}
	cursor += messageHeaderSize

	if routeLen > 0 {
		m.Route = string(b[cursor : cursor+routeLen])
		cursor += routeLen
	}

	if payloadLen > 0 {
		m.Payload = b[cursor : cursor+payloadLen]
	}
	return m, nil

}

//UnmarshalHeader is unsafe read expect at least 7 bytes length
func UnmarshalHeader(b []byte) (code byte, nameLen, routeLen, payloadLen uint16) {
	code = b[0]
	nameLen = binary.BigEndian.Uint16(b[1:3])
	routeLen = binary.BigEndian.Uint16(b[3:5])
	payloadLen = binary.BigEndian.Uint16(b[5:7])
	return
}

//Len calcualte current length of message in bytes
func (m *Message) Len() int {
	return messageHeaderSize + len(m.Name) + len(m.Route) + len(m.Payload)
}
