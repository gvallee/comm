/*
 * Copyright (c) 2019 Geoffroy Vallee, All rights reserved
 * This software is licensed under a 3-clause BSD license. Please consult the
 * LICENSE.md file distributed with the sources of this project regarding your
 * rights to use or distribute this software.
 */

package comm

import (
	"log"
	"testing"

	"github.com/gvallee/comm/pkg/transport"
)

const (
	tcpServerURL = "127.0.0.1"
	msgStr       = "Hello World"
)

func magicRecvRoutine(t *testing.T) {
	// Create an 'auto' engine, which means the engine will setup everything required
	// to accept connections and perform communications without having the user specifying
	// much details about the system. In this mode, the engine enters in discovery mode,
	// setup all required mechanism to any type of communications
	engineCfg := EngineCfg{
		Mode: Auto,
	}
	commEngine := engineCfg.Init()
	if commEngine == nil {
		t.Fatal("unable to start engine")
	}
}

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
	if tpt == nil {
		t.Fatalf("unable to add transport")
	}

	// Since accept is set to true, creating an endpoint with a single TCP transport
	// automatically create a Go routine that will accept incoming connections
	ep := commEngine.CreateEndpoint()
	if ep == nil {
		t.Fatal("unable to create endpoint")
	}

	log.Println("Server: receiving message...")
	msg := ep.Recv()
	if string(msg) != msgStr {
		t.Fatalf("Received %s instead of %s", string(msg), msgStr)
	}

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
	if tpt == nil {
		t.Fatal("unable to add transport")
	}

	// Use that endpoint to connect to server
	targetEP := tpt.Connect(tcpServerURL)
	if targetEP == nil {
		t.Fatalf("unable to connect to endpoint")
	}

	// Send a few messages
	msg := []byte(msgStr)

	// todo: do not rely on transport.Send but wait for a send completion event so we can check
	// how much data was sent
	err := targetEP.Send(msg)
	if err != nil {
		t.Fatal("failed to send message")
	}

	// This will emit a termination event and make sure everything is going to
	// be cleanly finalized
	tpt.Fini()
}

func TestMagicComm(t *testing.T) {
	go magicRecvRoutine(t)

	engineCfg := EngineCfg{
		Mode: Auto,
	}
	commEngine := engineCfg.Init()
	if commEngine == nil {
		t.Fatal("unable to create communication engine")
	}

	log.Println("Connection to endpoint on 127.0.0.1")
	ep := commEngine.Connect("127.0.0.1")
	if ep == nil {
		t.Fatal("unable to connect to remote endpoint")
	}
}
