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
	handler, err := NewAsyncHandler(testWriter, readCloser)
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
			handler.WriteNamed(byte(i), "ota", "path", m)
		}
	}
	time.Sleep(100 * time.Microsecond)
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
	handler.(*AsyncClient).shutdown()
}
