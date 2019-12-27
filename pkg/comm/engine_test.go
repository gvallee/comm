/*
 * Copyright (c) 2019 Geoffroy Vallee, All rights reserved
 * This software is licensed under a 3-clause BSD license. Please consult the
 * LICENSE.md file distributed with the sources of this project regarding your
 * rights to use or distribute this software.
 */

package comm

import (
	"testing"

	"github.com/gvallee/comm/pkg/transport"
)

const (
	tcpServerURL = "tcp://172.0.0.1"
	msgStr       = "Hello World"
)

func recvRoutine(t *testing.T) {

	// Create a minimalist engine, we do not want much by default since we will
	// add manually the TCP transport, which will in turn switch the engine to
	// active we will request to accept connection
	engineCfg := EngineCfg{
		Mode: Minimalist,
	}
	commEngine := engineCfg.Init()

	serverCfg := transport.TCPTransportCfg{
		Interface: tcpServerURL,
		PortLow:   33333,
		PortHigh:  44444,
		Accept:    true,
	}
	tcpTransport := serverCfg.Init()

	tpt := commEngine.AddTransport(tcpTransport)
	if tpt != nil {
		t.Fatalf("unable to add transport")
	}

	// Since accept is set to true, creating an endpoint with a single TCP transport
	// automatically create a Go routine that will accept incoming connections
	ep := commEngine.CreateEndpoint()
	if ep == nil {
		t.Fatal("unable to create endpoint")
	}

	msg := ep.Recv()
	if string(msg) != msgStr {
		t.Fatalf("Received %s instead of %s", string(msg), msgStr)
	}
	/*
		// Receive loop
		for {
			evt := ep.GetRXEvent()
			if IsTermEvent(evt) {
				return
			}

			data := evt.data
			t.Log("Recv'd data: %s\n", string(data))
		}
	*/

	tpt.Fini()
}

func TestBasicSendRecv(t *testing.T) {
	// Create recv routine acting as a server
	go recvRoutine(t)

	engineCfg := EngineCfg{
		Mode: Minimalist,
	}
	commEngine := engineCfg.Init()

	serverCfg := transport.TCPTransportCfg{
		Interface: tcpServerURL,
		PortLow:   33333,
		Accept:    false,
	}
	tcpTransport := serverCfg.Init()
	if tcpTransport == nil {
		t.Fatal("unable to initialize transport")
	}

	tpt := commEngine.AddTransport(tcpTransport)
	if tpt != nil {
		t.Fatal("unable to add transport")
	}

	// Use that endpoint to connect to server
	targetEP := tpt.Connect()
	if targetEP != nil {
		t.Fatalf("unable to connect to endpoint")
	}

	// Send a few messages
	msg := []byte(msgStr)
	n, err := targetEP.Send(msg)
	if err != nil {
		t.Fatal("failed to send message")
	}
	if n != uint64(len(msg)) {
		t.Fatalf("send %d bytes instead of %d", int(n), len(msg))
	}

	// This will emit a termination event and make sure everything is going to
	// be cleanly finalized
	tpt.Fini()
}
