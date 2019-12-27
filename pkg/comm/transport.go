/*
 * Copyright (c) 2019 Geoffroy Vallee, All rights reserved
 * This software is licensed under a 3-clause BSD license. Please consult the
 * LICENSE.md file distributed with the sources of this project regarding your
 * rights to use or distribute this software.
 */

// This file implements everything required to abstract concrete transports
package comm

import (
	"fmt"
	"log"

	"github.com/gvallee/comm/pkg/transport"
	"github.com/gvallee/event/pkg/event"
)

const (
	/* event system constants */

	defaultNumEvents = 1024
	termEventTypeID  = "comm:evt:term"

	/* Tansport Modes */

	// AutoTransportMode identifies the automatic mode where the system will
	// implicitly detect the best transport for any given endpoint
	AutoTransportMode = "auto"

	// ExplicitTransportMode identifies the mode where a specific transport,
	// and no other, is requested
	ExplicitTransportMode = "explicit"

	/* Connection modes */

	// MultiplexConnectionMode specifies that connections can be used to
	// reach different endpoints
	MultiplexConnectionMode = "multiplex"

	// ParallelConnectionMode specifies that multiple connections can be used
	// to reach a single endpoint
	ParallelConnectionMode = "parallel"

	// SingleConnectionMode specifies that a single connection must be used
	// between any two given endpoints
	SingleConnectionMode = "single"
)

// Cfg represents the configuration of a transport
type Cfg struct {
	ID             string
	TransportMode  string
	ConnectionMode string
}

//type ConnManager interface{}

// TransportPriv is a data structure that is specific to a given transport implementation
//type TransportPriv interface{}

// SendFn is a function pointer representing a send function for a given transport`
type SendFn func() (uint64, error)

// RecvFn is a function pointer representing a receive function for a given transport
type RecvFn func() (uint64, error)

// CloseConnectionFn is a function
type CloseConnectionFn func() error

// Transport is the structure representing a given network transport
type Transport struct {
	cfg Cfg

	// todo: move to engine
	EventEngine *event.Engine

	// todo: move to engine
	EventTypes map[string]*event.EventType
	Priv       interface{}
	ConcreteID string

	//ConnMgr          ConnManager
	//Send             SendFn
	//Recv             RecvFn
	Close            CloseConnectionFn
	InitialNumEvents uint64

	// Pointers to the different transport we support, used to get transport specific data
	// Remember that this structure is associated to one and only one concrete transport (e.g., TCP).
	// However, an engine can be using many different transports at the same time, including some
	// of the same concrete transport.
	TCP *transport.TCPTransport
}

func (t *Transport) initEvtSystem() error {
	queueCfg := event.QueueCfg{
		Size: t.InitialNumEvents,
	}

	if t.InitialNumEvents == 0 {
		t.InitialNumEvents = defaultNumEvents
	}

	t.EventEngine = queueCfg.Init()
	if t.EventEngine == nil {
		return fmt.Errorf("unable to create event engine")
	}

	termEventType, err := t.EventEngine.NewType(termEventTypeID)
	if err != nil {
		return fmt.Errorf("unable to create new event type: %w", err)
	}
	t.EventTypes[termEventTypeID] = &termEventType

	return nil
}

// Init creates a new transport based on a requested configuration
func Init(cfg Cfg) *Transport {
	var t Transport
	t.cfg = cfg
	return &t
}

// Fini finalizes a given transport
func (t *Transport) Fini() error {
	evt := t.EventEngine.GetEvent(true)
	if evt == nil {
		return fmt.Errorf("unable to get event")
	}
	evt.EventType = *(t.EventTypes[termEventTypeID])
	err := evt.Emit(nil)
	if err != nil {
		return fmt.Errorf("failed to emit termination event: %w", err)
	}

	return nil
}

// Add sets a given concrete transport (e.g., TCP) to a generic transport structure
func (t *Transport) Add(tpt interface{}) error {
	switch tpt.(type) {
	case *transport.TCPTransport:
		if t.TCP != nil {
			return fmt.Errorf("TCP transport already defined")
		}
		t.TCP = tpt.(*transport.TCPTransport)
		t.Priv = tpt
		t.ConcreteID = transport.TCPTransportID
	default:
		return fmt.Errorf("unknown transport type")
	}

	return nil
}

// Send sends a message over a transport
func (t *Transport) Send(epID string, msg []byte) error {
	switch t.ConcreteID {

	case transport.TCPTransportID:
		tx := t.TCP.TxPool.Get()
		if tx == nil {
			return fmt.Errorf("unable to get a RX ")
		}
		hdr := transport.TCPHeader{
			MsgType: "INTERNAL:DATAMSG",
			EPid:    epID,
		}
		t.TCP.SendMsg(tx, hdr, msg)

	default:
		fmt.Errorf("unknown transport type: %s", t.ConcreteID)
	}
	return nil
}

// Recv receives a message from a transport
func (t *Transport) Recv() []byte {
	switch t.ConcreteID {

	case transport.TCPTransportID:
		rx := <-t.TCP.RecvQueue
		// Create a new event and emit it for the endpoint as a recv event

	default:
		log.Printf("[ERROR:transport] unknown transport type: %s", t.ConcreteID)
		return nil
	}
	return nil
}

func (tpt *Transport) Connect() *Endpoint {
	var ep Endpoint
	switch tpt.ConcreteID {
	case transport.TCPTransportID:
		// Add the transport to the endpoint
		ep.transports = append(ep.transports, *tpt)
		tpt.TCP.Conn = tpt.TCP.Connect(ep.ID)
	default:
		log.Printf("[ERROR:transport] unknown transport type: %s", tpt.ConcreteID)
		return nil
	}

	return &ep
}
