package fdstream

import (
	"errors"
	"fmt"
)

//Router is routing message by const route names
type Router struct {
	name     string
	inputQ   <-chan *Message
	outputQ  chan *Message
	routings map[string]reciver
}
type reciver struct {
	input  chan<- *Message
	output chan *Message
}

//AddRouting is a way to add new routings with input and output
//output can be nil in case one way routings only send without re
func (r *Router) AddRouting(name string, in chan<- *Message, out chan *Message) error {
	if len(name) == 0 || in == nil {
		return errors.New("wrong routing income data")
	}
	if _, ok := r.routings[name]; ok {
		return errors.New("routing with specified name already exists")
	}

	r.routings[name] = reciver{
		input:  in,
		output: out,
	}

	//Is one way chan or not we can expecte no routings reciver
	if out != nil {
		//The gorutine send responce from reciver to main chan
		go func(output <-chan *Message) {
			for m := range output {
				r.outputQ <- m
			}
		}(out)
	}
	return nil
}

//Route is constant router
//it is so solution implementation but do not supported vars inside route like HttpRoute
func (r *Router) route(m *Message) {
	if endpoint, ok := r.routings[m.Route]; ok {
		endpoint.input <- m
	} else {
		r.outputQ <- &Message{
			Code:    erMissRoutingCode,
			Route:   m.Route,
			Name:    m.Name,
			Payload: []byte(fmt.Sprintf("Routing is not found for name: %s router: %s", string(m.Route), r.name)),
		}
	}
}

//Start will start routings functionality. After this we should not add new routings
func (r *Router) Start() {
	go func(r *Router) {
		for m := range r.inputQ {
			r.route(m)
		}
	}(r)
}

//NewRouter create new router for messages with input and output chans
//The router will not started. Please add routings at first and than call start.
// name should not be empty
func NewRouter(name string, input <-chan *Message, output chan *Message) (*Router, error) {
	if input == nil || output == nil {
		return nil, errors.New("Wrong routing communication chans")
	}
	if len(name) == 0 {
		return nil, errors.New("Miss name for router")
	}
	router := &Router{
		name:     name,
		inputQ:   input,
		outputQ:  output,
		routings: make(map[string]reciver, 10),
	}
	return router, nil
}

//NewRouterForClient create router for async communication client
// name should be not empty
// client should not be empty
func NewRouterForClient(name string, client *AsyncClient) (*Router, error) {
	if client == nil {
		return nil, errors.New("Client is not specified")
	}
	return NewRouter(name, client.toReadMessageQ, client.toSendMessageQ)
}
