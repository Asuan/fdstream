package fdstream

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSyncWrite(t *testing.T) {
	as := assert.New(t)

	readCloser := &TestReaderWaiter{
		d: time.Duration(1 * time.Second), //Wait reader for test writer
	}

	testWriter := &TestWriteCloser{
		m: map[int][]byte{},
	}
	handler, err := NewSyncClient(testWriter, readCloser, time.Duration(2*time.Second))
	as.Nil(err)

	testMessages := [10]*Message{}

	for i := 0; i < 10; i++ {
		m := &Message{
			Code:    byte(i),
			Name:    "name" + strconv.Itoa(i),
			Payload: []byte("value" + strconv.Itoa(i)),
		}
		testMessages[i] = m
		if i%2 == 0 {
			handler.Write(m)
		} else {
			handler.WriteNamed(byte(i), "ota", m)
		}
	}
	time.Sleep(400 * time.Microsecond)
	as.Equal(10, testWriter.counter)
	for _, reciaveMessages := range testWriter.m {
		exist := false
		for _, sended := range testMessages {
			if bytes.Contains(reciaveMessages, sended.Payload) && bytes.Contains(reciaveMessages, []byte(sended.Name)) {
				exist = true
				break
			}
		}
		as.True(exist)
	}
	handler.Shutdown() //Stop loops
}

func TestSyncRead(t *testing.T) {
	as := assert.New(t)
	data, _ := (&Message{"name", 0, byte(0), []byte("anry")}).Marshal()
	readCloser := &TestReaderWaiter{
		data: data,
		d:    time.Duration(200 * time.Millisecond), //Wait reader for test writer
	}

	testWriter := &TestWriteCloser{
		m: map[int][]byte{},
	}
	handler, err := NewSyncClient(testWriter, readCloser, time.Duration(2*time.Second))
	as.Nil(err)

	m, err := handler.read(0)
	as.Nil(err)
	as.Equal(byte(0), m.Code)
	as.Equal("name", m.Name)

	as.Equal([]byte("anry"), m.Payload)

	handler.Shutdown() //Stop loops

}
