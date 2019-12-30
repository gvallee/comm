/*
 * Copyright (c) 2019 Geoffroy Vallee, All rights reserved
 * This software is licensed under a 3-clause BSD license. Please consult the
 * LICENSE.md file distributed with the sources of this project regarding your
 * rights to use or distribute this software.
 */

package transport

import (
	"fmt"
	"log"
	"testing"
)

const (
	smServerID = "i am the server"
	smClientID = "i am the client"
	smMsg1     = "message 1"
	smMsg2     = "message 2"
)

func doSMServer(t *testing.T, tpt *SMTransport) {
	data := tpt.Recv(smClientID)
	if data == nil {
		t.Fatal("unable to recv message")
	}
	if string(data[0:len(smMsg1)]) != smMsg1 {
		t.Fatalf("Recv'd %s instead of %s", string(data), smMsg1)
	}
	fmt.Printf("[SM server] Data received: %s\n", string(data))

	data = tpt.Recv(smClientID)
	if data == nil {
		t.Fatal("unable to recv message")
	}
	if string(data[0:len(smMsg2)]) != smMsg2 {
		t.Fatalf("Recv'd %s instead of %s", string(data), smMsg2)
	}
	fmt.Printf("[SM server] Data received: %s\n", string(data))

	log.Println("[SM server] Sending term msg")
	err := tpt.Send(smClientID, []byte("all done"))
	if err != nil {
		t.Fatalf("unable to send termination message: %s", err)
	}

	tpt.Disconnect(smServerID)
}

func doSMClient(t *testing.T, tpt *SMTransport) {
	err := tpt.Send(smServerID, []byte(smMsg1))
	if err != nil {
		t.Fatalf("unable to send message: %s", err)
	}

	err = tpt.Send(smServerID, []byte(smMsg2))
	if err != nil {
		t.Fatalf("unable to send message: %s", err)
	}

	data := tpt.Recv(smServerID)
	if string(data[:8]) != "all done" {
		t.Fatalf("failed to receive term msg; recv'd %s instead of 'all done'\n", string(data[:8]))
	}

	tpt.Disconnect(smClientID)
}

func TestSM(t *testing.T) {
	cfg := SMTransportCfg{
		peer1:     smServerID,
		peer2:     smClientID,
		blockSize: 4096,
		numBlocks: 512,
	}

	// Initialization
	tpt := cfg.Init()
	if tpt == nil {
		t.Fatal("unable to initialize a SM transport")
	}

	// Create two threads: one that reads, one that writes
	go doSMServer(t, tpt)
	doSMClient(t, tpt)

	tpt.Fini()
}
