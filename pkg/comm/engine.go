/*
 * Copyright (c) 2019 Geoffroy Vallee, All rights reserved
 * This software is licensed under a 3-clause BSD license. Please consult the
 * LICENSE.md file distributed with the sources of this project regarding your
 * rights to use or distribute this software.
 */

package comm

import (
	"log"

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
	Mode string
}

// Engine is a structure representing a communication engine. It is allowed to have multiple
// communication engine per application.
type Engine struct {
	transports  []*Transport
	cfg         EngineCfg
	eventEngine event.Engine
	eps         map[string]*Endpoint
}

// Init creates a new configuration engine from a given configuration
func (cfg *EngineCfg) Init() *Engine {
	var e Engine
	e.cfg = *cfg
	e.eps = make(map[string]*Endpoint)
	return &e
}

// AddTransport adds a transport to a given communication engine.
func (e *Engine) AddTransport(tpt interface{}) *Transport {
	if e == nil || tpt == nil {
		return nil
	}

	var newTransport Transport
	err := newTransport.Add(tpt)
	if err != nil {
		log.Println("[ERROR:engine] unable to add transport")
		return nil
	}
	e.transports = append(e.transports, &newTransport)

	return &newTransport
}

func (e *Engine) getNextTransport() int {
	// todo: do not only use the first transport available
	return 0
}

// GetEvent returns a event from the queue of inactive events of a given engine
func (e *Engine) GetEvent() event.Event {
	return *(e.eventEngine.GetEvent(true))
}
