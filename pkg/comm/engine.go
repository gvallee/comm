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
	"strings"

	"github.com/gvallee/comm/internal/pkg/util"
	"github.com/gvallee/event/pkg/event"
)

const (
	// Minimalist is the mode where the communication engine will not set extra resources
	// to enable a high level of communication abstraction and advanced semantics. For example,
	// endpoints are not automatically listening for incoming connections of all interface that
	// can be detected.
	Minimalist = "engine:mode:minimalist"

	// Auto enables a high level of semantics by setting up advanced features such as listening
	// on all interface for TCP connection and ease the creation of connections between endpoints
	// and the setup of advanced configurations.
	Auto = "engine:mode:auto"
)

// EngineCfg is a structure representing the configuration of a given engine`
type EngineCfg struct {
	// Mode of the engine, e.g., 'Auto' or 'Minimalist'.
	Mode string
}

// Engine is a structure representing a communication engine. It is allowed to have multiple
// communication engine per application.
type Engine struct {
	ifaces      []util.NetIface
	transports  []*Transport
	cfg         EngineCfg
	eventEngine event.Engine
	eps         map[string]*Endpoint
}

func (e *Engine) initResourceDiscovery() error {
	var err error
	e.ifaces, err = util.GetLocalInferfaces()
	if err != nil {
		return fmt.Errorf("unable to detect local network interfaces: %w", err)
	}

	// For all IPv4 addresses, initiate a TCP transport and a thread to accept incoming
	// connections using a default port
	for _, iface := range e.ifaces {
		if strings.HasSuffix(iface.Addr, "/8") || strings.HasSuffix(iface.Addr, "/24") {
			ip := strings.TrimSuffix(iface.Addr, "/8")
			ip = strings.TrimSuffix(iface.Addr, "/24")
			// todo: we clearly need the IP but do we need to save the interface as well to ease
			// later lookup in order to find the interface to perform a connect?
			tpt := e.createAutoTCPTransport(iface, ip)
			if tpt == nil {
				return fmt.Errorf("unable to instantiate a TCP transport for %s", ip)
			}
		}
	}

	return nil
}

// Init creates a new configuration engine from a given configuration
func (cfg *EngineCfg) Init() *Engine {
	var e Engine
	e.cfg = *cfg
	e.eps = make(map[string]*Endpoint)

	if cfg.Mode == Auto {
		err := e.initResourceDiscovery()
		if err != nil {
			log.Println("[ERROR:engine] unable to detect local network interfaces: %w", err)
			return nil
		}
	}

	return &e
}

// AddTransport adds a transport to a given communication engine.
func (e *Engine) AddTransport(tpt interface{}) *Transport {
	if e == nil || tpt == nil {
		log.Printf("[ERROR:engine] invalid parameter(s); unable to add transport")
		return nil
	}

	newTransportCfg := TransportCfg{
		ID:             "",
		TransportMode:  "",
		ConnectionMode: "",
	}
	newTransport := newTransportCfg.Init()

	err := newTransport.Add(tpt)
	if err != nil {
		log.Printf("[ERROR:engine] unable to add transport: %s", err)
		return nil
	}
	e.transports = append(e.transports, newTransport)
	newTransport.commEngine = e

	return newTransport
}

func (e *Engine) getNextTransport() int {
	// todo: do not only use the first transport available
	return 0
}

// GetEvent returns a event from the queue of inactive events of a given engine
func (e *Engine) GetEvent() event.Event {
	return *(e.eventEngine.GetEvent(true))
}

func (e *Engine) createEndpointForIface(iface util.NetIface, ip string) *Endpoint {
	tpt := e.getTransportFromIface(iface)
	if tpt == nil {
		log.Printf("[ERROR:engine] Unable to get transport for %s\n", iface.Name)
		return nil
	}

	// Use that endpoint to connect to server
	targetEP := tpt.Connect(ip)
	if targetEP == nil {
		log.Println("[ERROR:engine] Unable to connect to endpoint")
		return nil
	}

	return targetEP
}

// Connect will establish a connection to a remote endpoint. This function
// is meant to be used with a communication engine in 'Auto' mode.
func (e *Engine) Connect(id string) *Endpoint {
	if e == nil || e.cfg.Mode != Auto {
		log.Println("[ERROR:engine] invalid engine")
		return nil
	}

	// Try to find a network interface we can use
	for _, iface := range e.ifaces {
		if strings.Contains(iface.Addr, id) || util.SameNetwork(iface.Addr, id) {
			// The exact same interface is present locally,
			// we can connect using the default connection values (e.g., port)
			ep := e.createEndpointForIface(iface, id)
			if ep == nil {
				// We are unable to create the endpoint, we try with the next network interface
				continue
			}

			return ep
		}
	}

	// We did not manage to find a suitable network interface to connect to the
	// remote endpoint
	return nil
}
