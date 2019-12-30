package transport

import (
	"log"
	"testing"
)

const (
	tcpClientID   = "I am the client"
	tcpMsg1       = "message 1"
	tcpMsg2       = "message 2"
	tcpAllDoneMsg = "All done."
)

func doTCPServer(t *testing.T) {
	log.Println("Hello, i am the server test")
	cfg := TCPTransportCfg{
		Interface: "127.0.0.1",
		PortLow:   44444,
		PortHigh:  44444,
		Accept:    true,
	}
	tcp := cfg.Init()
	if tcp == nil {
		t.Fatalf("unable to instantiate TCP transport")
	}

	log.Println("Server test: Connection accepted")

	rx := <-tcp.RecvQueue
	data, err := tcp.ExtractPayload(rx)
	if err != nil {
		t.Fatalf("unable to extract payload: %s", err)
	}
	if string(data) != tcpMsg1 {
		t.Fatalf("receiver %s instead of %s", string(data), tcpMsg1)
	}
	log.Printf("Successfully received: %s\n", string(data))
	err = tcp.RxPool.Return(rx)
	if err != nil {
		t.Fatalf("unable to return RX: %s", err)
	}

	rx = <-tcp.RecvQueue
	data, err = tcp.ExtractPayload(rx)
	if err != nil {
		t.Fatalf("unable to extract payload: %s", err)
	}
	if string(data) != tcpMsg2 {
		t.Fatalf("receiver %s instead of %s", string(data), tcpMsg2)
	}
	log.Printf("Successfully received: %s\n", string(data))
	err = tcp.RxPool.Return(rx)
	if err != nil {
		t.Fatalf("unable to return RX: %s", err)
	}

	hdr := TCPHeader{
		MsgType: DATAMSG,
		Dst:     tcpClientID,
	}
	err = tcp.SendMsg(hdr, []byte(tcpAllDoneMsg))
	if err != nil {
		t.Fatal("unable to send message")
	}
}

func doTCPClient(t *testing.T) {
	id := tcpClientID
	log.Printf("(%s) Hello, i am a test client\n", id)
	cfg := TCPTransportCfg{
		Interface: "127.0.0.1",
		PortLow:   44444,
		Accept:    false,
	}
	tcp := cfg.Init()
	if tcp == nil {
		t.Fatalf("unable to instantiate TCP transport")
	}

	log.Printf("(%s) Connecting to server...", id)
	serverID, err := tcp.Connect(tcpClientID, "127.0.0.1")
	if err != nil {
		t.Fatalf("connect failed: %s", err)
	}

	log.Printf("(%s) Connection to %s successded.\n", id, serverID)
	log.Printf("(%s) Start sending test messages...", id)

	hdr := TCPHeader{
		MsgType: DATAMSG,
	}
	err = tcp.SendMsg(hdr, []byte(tcpMsg1))
	if err != nil {
		t.Fatalf("unable to send first message: %s", err)
	}
	err = tcp.SendMsg(hdr, []byte(tcpMsg2))
	if err != nil {
		t.Fatalf("unable to send second message: %s", err)
	}

	for {
		log.Printf("(%s) Waiting for 'all done' message from server...", id)
		rx := <-tcp.RecvQueue
		data, err := tcp.ExtractPayload(rx)
		if err != nil {
			t.Fatalf("unable to extract payload: %s", err)
		}
		if string(data) == tcpAllDoneMsg {
			log.Println("Successfully receive the 'all done' message from server")
			break
		} else {
			t.Fatalf("(%s) Received %s instead of the 'all done' message", id, string(data))
		}
	}

	err = tcp.SendTermMsg(tcpClientID, serverID)
	if err != nil {
		t.Fatalf("unable to send termination message: %s", err)
	}

	tcp.Close()
}

func TestTCP(t *testing.T) {
	go doTCPServer(t)

	doTCPClient(t)
}
