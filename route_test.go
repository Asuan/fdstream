package fdstream

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newRoute(as *assert.Assertions) *Router {
	income := make(chan *Message, 5)
	outcome := make(chan *Message, 5)
	router, err := NewRouter("test router", income, outcome)
	as.Nil(err, "Error on creating router")
	return router
}
func TestRouter_AddRouting(t *testing.T) {
	as := assert.New(t)
	router := newRoute(as)
	tests := []struct {
		name    string
		wantErr bool
		in      chan *Message
		out     chan *Message
	}{
		{
			name:    "simple correct router",
			wantErr: false,
			in:      make(chan *Message, 5),
			out:     make(chan *Message, 5),
		}, {
			name:    "simple one way router",
			wantErr: false,
			in:      make(chan *Message, 5),
			out:     nil,
		}, {
			name:    "no chan router",
			wantErr: true,
			in:      nil,
			out:     nil,
		}, {
			name:    "only out chan router",
			wantErr: true,
			in:      nil,
			out:     make(chan *Message, 5),
		}, {
			name:    "simple correct router",
			wantErr: true,
			in:      make(chan *Message, 5),
			out:     make(chan *Message, 5),
		}, {
			name:    "",
			wantErr: true,
			in:      make(chan *Message, 5),
			out:     make(chan *Message, 5),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := router.AddRouting(tt.name, tt.in, tt.out)
			as.Equal(tt.wantErr, (err != nil), "Wrong expected result")
		})
	}
}

func TestRouter_Route(t *testing.T) {
	as := assert.New(t)
	router := newRoute(as)
	in1, in2, out1, out2 := make(chan *Message, 2), make(chan *Message, 2), make(chan *Message, 2), make(chan *Message, 2)

	router.AddRouting("1", in1, out1)
	router.AddRouting("2", in2, out2)

	tests := []struct {
		name    string
		wantErr bool
		message *Message
		check   chan *Message
		resp    chan *Message
	}{
		{
			name:    "first message",
			message: &Message{Route: "1", Name: "route to 1, 1"},
			check:   in1,
			resp:    out1,
		}, {
			name:    "second message",
			message: &Message{Route: "1", Name: "route to 1, 2"},
			check:   in1,
			resp:    out1,
		}, {
			name:    "thrid message",
			message: &Message{Route: "2", Name: "route to 2"},
			check:   in2,
			resp:    out2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router.route(tt.message)
			time.Sleep(100 * time.Millisecond)
			as.Len(tt.check, 1)
			m := <-tt.check
			as.Equal(tt.message.Route, m.Route)
			as.Contains(tt.message.Name, m.Name)
			tt.resp <- m
			time.Sleep(100 * time.Millisecond)
			as.Len(router.outputQ, 1)
			m1 := <-router.outputQ
			as.Equal(tt.message.Route, m1.Route)
			as.Contains(tt.message.Name, m1.Name)
		})
	}

	//Miss routing case
	router.route(&Message{Route: "3", Name: "error"})
	time.Sleep(100 * time.Millisecond)
	as.Len(router.outputQ, 1)
	m1 := <-router.outputQ
	as.Equal("3", m1.Route)
	as.Equal(erMissRoutingCode, m1.Code)
	as.Contains("error", m1.Name)
}
