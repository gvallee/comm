/*
 * Copyright (c) 2019 Geoffroy Vallee, All rights reserved
 * This software is licensed under a 3-clause BSD license. Please consult the
 * LICENSE.md file distributed with the sources of this project regarding your
 * rights to use or distribute this software.
 */

package comm

import (
	"fmt"
	"log"

	"github.com/gvallee/comm/internal/pkg/util"
	"github.com/gvallee/comm/pkg/transport"
	"github.com/gvallee/event/pkg/event"
)

const (
	defaultEPNumEvts = 4096

	userDataEventTypeID = "ep:evt:data"
)

// Endpoint is a structure representing an endpoint
type Endpoint struct {
	conns       []Connection
	transports  []Transport
	engine      *Engine
	eventTypes  map[string]*event.EventType
	eventEngine *event.Engine

	// ID is the locally unique endpoint identifier (256-character string)
	ID string

	// RXEvent is a queue of events where events associated to receive completion are stored
	// and ready for the application to use
	RXEvents chan event.Event
}

// Send sends a message to a given endpoint
func (ep *Endpoint) Send(data []byte) error {
	// todo: do not only use the first transport=
	return ep.transports[0].Send(ep.ID, data)
}

// Recv receives a message from a given endpoint
func (ep *Endpoint) Recv() []byte {
	evt := <-ep.RXEvents
	// We copy the data to end it to the application
	data := make([]byte, len(evt.Data[0]))
	copy(data, evt.Data[0])
	err := ep.eventEngine.Return(&evt)
	if err != nil {
		return nil
	}
	return data
}

/*
// ReturnEvent returns an event to the inactive queue of the engine to which the endpoint is associated
func (ep *Endpoint) ReturnEvent() error {
	return nil
}
*/

// Connect connects to a given endpoint using a specific target transport
func (ep *Endpoint) Connect(target interface{}) *Endpoint {

	return nil
}

// Disconnect ends all connections for a given endpoint
func (ep *Endpoint) Disconnect(target *Endpoint) error {
	success := true
	for _, conn := range ep.conns {
		err := conn.Close()
		if err != nil {
			// If a close() fails, we still want to try to close other connection
			// This is for example valuable when the connection is already closed
			// because the endpoint on the other side closed it.
			success = false
		}
	}
	ep = nil

	if !success {
		return fmt.Errorf("failed to close all connection")
	}

	return nil
}

// LookupEP returns the endpoint structure based on a endpoint unique ID
func (e *Engine) LookupEP(epID string) *Endpoint {
	return e.eps[epID]
}

// eventThread checks all transports associated to the endpoint and if a receive
// completed (i.e., a RX buffer is available), extract the information from it
// and emit an associated event.
func eventThread(ep *Endpoint) {
	for _, tpt := range ep.transports {
		if tpt.ConcreteID == transport.TCPTransportID {
			if tpt.TCP != nil && len(tpt.TCP.RecvQueue) > 0 {
				rx := <-tpt.TCP.RecvQueue
				evt := ep.engine.eventEngine.GetEvent(true)
				if evt != nil {
					log.Println("[ERROR:eventThread] unable to get event")
				}
				evt.SetType(userDataEventTypeID)
				evt.Data[0] = tpt.TCP.GetPayloadFromRX(rx)
				// At this point we have the data, we can release the RX
				tpt.TCP.RecvQueue <- rx
				ep.RXEvents <- *evt
			}
		}
	}
}

func (ep *Endpoint) registerDefaultEvtTypes() error {
	userDataType, err := ep.eventEngine.NewType(userDataEventTypeID)
	if err != nil {
		return err
	}
	ep.eventTypes[userDataEventTypeID] = &userDataType
	return nil
}

// CreateEndpoint returns an endpoint in the context of a given engine
func (e *Engine) CreateEndpoint() *Endpoint {
	// Parse all transports, find the one with the highest priority and create
	// endpoint based on the transport configuration
	if e == nil {
		return nil
	}

	var ep Endpoint
	ep.engine = e
	ep.ID = util.GenerateID()
	// We need an ID that is locally unique
	for {
		if e.eps[ep.ID] == nil {
			break
		}
		ep.ID = util.GenerateID()
	}
	e.eps[ep.ID] = &ep

	// Initialize the event system specific to the endpoint
	evtEngineCfg := event.QueueCfg{
		Size: defaultEPNumEvts,
	}
	ep.eventEngine = evtEngineCfg.Init()
	ep.eventTypes = make(map[string]*event.EventType)
	ep.RXEvents = make(chan event.Event)

	// Create the event thread
	go eventThread(&ep)

	// Find the active transport and those that do not have a thread to accept connection
	// and need one
	for _, t := range e.transports {
		if t.ConcreteID == transport.TCPTransportID && t.TCP.Cfg.Accept && !t.TCP.IsAcceptingConns() {
			// Associate the TCP transport accepting connection to the new endpoint
			ep.transports = append(ep.transports, *t)
			log.Println("[INFO:endpoint] creating accept thread for TCP transport")
		}
	}

	return &ep
}
