package transport

import (
	"log"
	"testing"
)

const (
	clientID   = "I am the client"
	serverID   = "I am the server"
	msg1       = "message 1"
	msg2       = "message 2"
	allDoneMsg = "All done."
)

func doServer(t *testing.T) {
	log.Println("Hello, i am the server test")
	cfg := TCPTransportCfg{
		Interface: "127.0.0.1",
		PortLow:   44444,
		PortHigh:  44445,
		Accept:    true,
	}
	tcp := cfg.Init()
	if tcp == nil {
		t.Fatalf("unable to instantiate TCP transport")
	}

	err := tcp.Accept(serverID)
	if err != nil {
		t.Fatalf("unable to accept incoming connections: %s", err)
	}

	log.Println("Server test: Connection accepted")

	rx := <-tcp.RecvQueue
	data, err := extractPayload(rx)
	if err != nil {
		t.Fatalf("unable to extract payload: %s", err)
	}
	if string(data) != msg1 {
		t.Fatalf("receiver %s instead of %s", string(data), msg1)
	}
	log.Printf("Successfully received: %s\n", string(data))
	err = tcp.RxPool.Return(rx)
	if err != nil {
		t.Fatalf("unable to return RX: %s", err)
	}

	rx = <-tcp.RecvQueue
	data, err = extractPayload(rx)
	if err != nil {
		t.Fatalf("unable to extract payload: %s", err)
	}
	if string(data) != msg2 {
		t.Fatalf("receiver %s instead of %s", string(data), msg2)
	}
	log.Printf("Successfully received: %s\n", string(data))
	err = tcp.RxPool.Return(rx)
	if err != nil {
		t.Fatalf("unable to return RX: %s", err)
	}

	hdr := TCPHeader{
		MsgType: DATAMSG,
		Src:     serverID,
		Dst:     clientID,
	}
	err = tcp.SendMsg(hdr, []byte(allDoneMsg))
	if err != nil {
		t.Fatal("unable to send message")
	}
}

func doClient(t *testing.T) {
	log.Println("Hello, i am the test client")
	cfg := TCPTransportCfg{
		Interface: "127.0.0.1",
		PortLow:   44444,
		Accept:    false,
	}
	tcp := cfg.Init()
	if tcp == nil {
		t.Fatalf("unable to instantiate TCP transport")
	}

	log.Println("Connecting to server...")
	err := tcp.Connect(clientID)
	if err != nil {
		t.Fatalf("connect failed: %s", err)
	}

	log.Println("Start sending test messages...")

	hdr := TCPHeader{
		MsgType: DATAMSG,
		Src:     clientID,
		Dst:     serverID,
	}
	err = tcp.SendMsg(hdr, []byte(msg1))
	if err != nil {
		t.Fatalf("unable to send first message: %s", err)
	}
	err = tcp.SendMsg(hdr, []byte(msg2))
	if err != nil {
		t.Fatalf("unable to send second message: %s", err)
	}

	for {
		log.Println("Waiting for 'all done' message from server...")
		rx := <-tcp.RecvQueue
		log.Println("RX buffer is ready...")
		data, err := extractPayload(rx)
		if err != nil {
			t.Fatalf("unable to extract payload: %s", err)
		}
		if string(data) == allDoneMsg {
			log.Println("Successfully receive the 'all done' message from server")
			break
		} else {
			t.Fatalf("Received %s instead of the 'all done' message", string(data))
		}
	}

	err = tcp.SendTermMsg(clientID, serverID)
	if err != nil {
		t.Fatalf("unable to send termination message: %s", err)
	}

	tcp.Close()
}

func TestTCP(t *testing.T) {
	go doServer(t)

	doClient(t)
}
