package fdstream

import (
	"bytes"
	"io/ioutil"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type TestWriteCloser struct {
	counter int
	m       map[int][]byte
}

type TestReaderWaiter struct {
	d time.Duration
}

func (t *TestReaderWaiter) Read(b []byte) (int, error) {
	time.Sleep(t.d)
	return 0, nil
}

func (t *TestReaderWaiter) Close() error {
	return nil
}

func (t *TestWriteCloser) Write(v []byte) (int, error) {
	t.counter++
	//t.m[t.counter] = v
	return ioutil.Discard.Write(v)
}

func (t *TestWriteCloser) Close() error {
	return nil
}

func TestWrite(t *testing.T) {
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
			handler.WriteNamed(byte(i), "ota", "", m)
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
