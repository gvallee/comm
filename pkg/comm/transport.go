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

	"github.com/gvallee/comm/internal/pkg/util"
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

	/* Some default values with use in the context of TCP */
	defaultTCPPortLow  = 50000
	defaultTCPPortHigh = 50100
)

// Cfg represents the configuration of a transport
type TransportCfg struct {
	// ID identifies the transport
	ID string

	// TransportMode specifies the transport's mode, i.e., 'auto' or
	// 'explicit'.
	// This impacts how much details the calling application is providing and
	// assumption about configuration. For example, in 'auto' mode, the
	// transport falls back in discovery mode and is able to accept of broad
	// range of connections
	TransportMode string

	// ConnectionMode specifies how a transport can be used by a
	// communication engine.
	// For example, in 'multiplex' mode, a single connection can be used to
	// transfer message from/to different endpoints; in 'single' mode, a
	// connection can be used only for communications between a single pair
	// of endpoints; while the 'parallel' mode allows for multiple connections
	// between a pair of endpoints.
	ConnectionMode string
}

// Transport is the structure representing a given network transport
type Transport struct {
	cfg        TransportCfg
	iface      util.NetIface
	commEngine *Engine
	eps        map[string]*Endpoint

	// EventEngine is the event engine associated to the transport
	EventEngine *event.Engine

	// EventTypes are a dictionary of event types based on a human
	// readable identifier
	EventTypes map[string]*event.EventType

	// Priv is the underlying concrete transport (to be deprecated,
	// use ConcreteID instead)
	Priv interface{}

	// ConcreteID identifies the type of the underlying concrete
	// transport (e.g., 'TCP')
	ConcreteID string

	// InitialNumEvents specifies how many inactive events are
	// available in the transport's event system.
	// todo: move to TransportCfg
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
func (cfg *TransportCfg) Init() *Transport {
	var t Transport
	t.cfg = *cfg
	t.eps = make(map[string]*Endpoint)
	t.EventTypes = make(map[string]*event.EventType)
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

func addTCPTransport(t *Transport, tcp *transport.TCPTransport) error {
	if t == nil || tcp == nil {
		return fmt.Errorf("invalid parameter(s); cannot add tcp transport")
	}
	log.Println("Adding TCP transport...")
	if t.TCP != nil {
		return fmt.Errorf("TCP transport already defined")
	}
	t.Priv = tcp
	t.ConcreteID = transport.TCPTransportID
	t.TCP = tcp
	return nil
}

// Add sets a given concrete transport (e.g., TCP) to a generic transport structure
func (t *Transport) Add(tpt interface{}) error {
	switch actualTransport := tpt.(type) {
	case transport.TCPTransport:
		//tcp := tpt.(transport.TCPTransport)
		err := addTCPTransport(t, &actualTransport)
		if err != nil {
			return fmt.Errorf("failed to add TCP transport: %s", err)
		}
	case *transport.TCPTransport:
		/*
			tcp := tpt.(*transport.TCPTransport)
			if tcp == nil {
				log.Fatal("Mmmm....")
			}
		*/
		err := addTCPTransport(t, actualTransport)
		if err != nil {
			return fmt.Errorf("failed to add TCP transport: %s", err)
		}
	default:
		return fmt.Errorf("unknown transport type")
	}

	return nil
}

// Send sends a message over a transport
func (t *Transport) Send(epID string, msg []byte) error {
	switch t.ConcreteID {

	case transport.TCPTransportID:
		hdr := transport.TCPHeader{
			MsgType: "INTERNAL:DATAMSG",
			Src:     epID,
		}
		err := t.TCP.SendMsg(hdr, msg)
		if err != nil {
			return fmt.Errorf("unable to send TCP message")
		}

	default:
		return fmt.Errorf("unknown transport type: %s", t.ConcreteID)
	}
	return nil
}

// LookupReceiver looks into the list of all endpoints that are
// reachable using this transport, based on the endpoint identifier,
// and returns the associated endpoint structure.
func (t *Transport) LookupReceiver(target string) *Endpoint {
	return t.eps[target]
}

// Recv receives a message from a transport
func (t *Transport) Recv() []byte {
	switch t.ConcreteID {

	case transport.TCPTransportID:
		rx := <-t.TCP.RecvQueue
		// Create a new event and emit it for the endpoint as a recv event
		dst := t.TCP.ExtractDest(rx)
		ep := t.LookupReceiver(dst)
		if ep == nil {
			log.Println("unknown target endpoint")
			return nil
		}
		evt := ep.eventEngine.GetEvent(true)
		if evt == nil {
			log.Printf("unable to get an event")
			return nil
		}

		// Emit event
		payload := t.TCP.GetPayloadFromRX(rx)
		evt.Emit(payload)
	default:
		log.Printf("[ERROR:transport] unknown transport type: %s", t.ConcreteID)
		return nil
	}
	return nil
}

// Connect to a specific remote node identified by an identifier.
// For example, the id can be a TCP address.
func (tpt *Transport) Connect(id string) *Endpoint {
	if tpt == nil || tpt.commEngine == nil {
		log.Println("[ERROR:transport] corrupted transport")
		return nil
	}
	ep := tpt.commEngine.CreateEndpoint()
	switch tpt.ConcreteID {
	case transport.TCPTransportID:
		if tpt.TCP == nil {
			log.Println("[ERROR:transport] corrupt transport; cannot connect")
		}

		// Add the transport to the endpoint
		ep.transports = append(ep.transports, *tpt)
		serverID, err := tpt.TCP.Connect(ep.ID, id)
		tpt.eps[serverID] = ep
		if err != nil {
			log.Printf("[ERROR:transport] unable to connect to remote peer: %s", err)
			return nil
		}
	default:
		log.Printf("[ERROR:transport] unknown transport type: %s", tpt.ConcreteID)
		return nil
	}

	return ep
}

func (e *Engine) createAutoTCPTransport(iface util.NetIface, ip string) *Transport {
	log.Printf("Instantiating TCP transport for %s\n", ip)

	// The transport will automatically start listening on the default lower port
	tcpCfg := transport.TCPTransportCfg{
		PortLow:            defaultTCPPortLow,
		PortHigh:           defaultTCPPortHigh,
		Accept:             true,
		DoNotBlockOnAccept: true,
	}
	tcp := tcpCfg.Init()
	if tcp == nil {
		log.Println("[ERROR:transport] unable to instantiate TCP transport")
		return nil
	}

	newTransport := e.AddTransport(tcp)
	if newTransport == nil {
		log.Println("[ERROR:transport] unable to create new transport")
	}
	newTransport.iface.Name = iface.Name
	newTransport.iface.Addr = iface.Addr

	return newTransport
}

func (e *Engine) getTransportFromIface(iface util.NetIface) *Transport {
	// Loop over the list of transports we know and find the one matching the target interface
	for _, tpt := range e.transports {
		if iface.Addr == tpt.iface.Addr && iface.Name == tpt.iface.Name {
			return tpt
		}
	}

	return nil
}
