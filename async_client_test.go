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
	data []byte
	i    int
	d    time.Duration
}

func (t *TestReaderWaiter) Read(b []byte) (int, error) {
	time.Sleep(t.d)
	if t.data != nil {
		begin := t.i

		for ; (t.i-begin) < len(b) && t.i < len(t.data); t.i++ {
			b[t.i-begin] = t.data[t.i]
		}
		total := t.i - begin
		if t.i == len(t.data) {
			t.i = 0
		}
		return total, nil
	}
	return 0, nil
}

func (t *TestReaderWaiter) Close() error {
	return nil
}

func (t *TestWriteCloser) Write(v []byte) (int, error) {
	t.counter++
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
			handler.WriteNamed(byte(i), "ota", m)
		}
	}
	time.Sleep(100 * time.Microsecond)
	as.Equal(10, testWriter.counter)
	for _, receiveMessages := range testWriter.m {
		exist := false
		for _, s := range testMessages {
			if bytes.Contains(receiveMessages, s.Payload) && bytes.Contains(receiveMessages, []byte(s.Name)) {
				exist = true
				break
			}
		}
		as.True(exist)
	}
	handler.Shutdown()
}

func TestRead(t *testing.T) {
	as := assert.New(t)
	data, _ := (&Message{"name", 0, byte(0), []byte("anry")}).Marshal()
	readCloser := &TestReaderWaiter{
		data: data,
		d:    time.Duration(200 * time.Millisecond), //Wait reader for test writer
	}

	testWriter := &TestWriteCloser{
		m: map[int][]byte{},
	}
	handler, err := NewAsyncHandler(testWriter, readCloser)
	as.Nil(err)

	m := handler.Read()
	as.Equal(byte(0), m.Code)
	as.Equal("name", m.Name)

	as.Equal([]byte("anry"), m.Payload)

	handler.Shutdown()
}

func TestRestore(t *testing.T) {
	as := assert.New(t)
	data, _ := (&Message{"name", 0, byte(0), []byte("anry")}).Marshal()
	readCloser := &TestReaderWaiter{
		data: data,
		d:    time.Duration(200 * time.Millisecond), //Wait reader for test writer
	}

	testWriter := &TestWriteCloser{
		m: map[int][]byte{},
	}
	handler, err := NewAsyncHandler(testWriter, readCloser)
	as.Nil(err)

	m := handler.Read()
	as.Equal(byte(0), m.Code)
	as.Equal("name", m.Name)

	as.Equal([]byte("anry"), m.Payload)

	handler.Shutdown()
	as.False(handler.IsAlive())

	//Restore and read again
	data, _ = (&Message{"name", 0, byte(0), []byte("anry")}).Marshal()
	readCloser = &TestReaderWaiter{
		data: data,
		d:    time.Duration(200 * time.Millisecond), //Wait reader for test writer
	}

	testWriter = &TestWriteCloser{
		m: map[int][]byte{},
	}
	handler.Restore(testWriter, readCloser)
	as.True(handler.IsAlive())

	m = handler.Read()
	as.Equal(byte(0), m.Code)
	as.Equal("name", m.Name)

	as.Equal([]byte("anry"), m.Payload)

	handler.Shutdown()
}
