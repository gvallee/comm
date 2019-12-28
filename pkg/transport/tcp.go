/*
 * Copyright (c) 2019 Geoffroy Vallee, All rights reserved
 * This software is licensed under a 3-clause BSD license. Please consult the
 * LICENSE.md file distributed with the sources of this project regarding your
 * rights to use or distribute this software.
 */

package transport

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/gvallee/comm/internal/pkg/util"
	"github.com/gvallee/memory_pool/pkg/pool"
)

const (
	// TCPTransport identifies the TCP transport
	TCPTransportID = "TCP"

	defaultTCPMaxRetry = 5
	defaultNumRX       = 1024
	defaultNumTX       = 1024
	defaultMTU         = 4096

	/* Message type specific constants */
	msgTypeLen     = 16
	srcLen         = 256
	dstLen         = 256
	sizeOfSizeLen  = 8
	payloadSizeLen = binary.MaxVarintLen64

	/* Offsets with a message/RX */
	msgTypeOffset     = 0
	srcOffset         = msgTypeOffset + msgTypeLen
	dstOffset         = srcOffset + srcLen
	sizeOfSizeOffset  = dstOffset + dstLen
	payloadSizeOffset = sizeOfSizeOffset + sizeOfSizeLen
	payloadOffset     = payloadSizeOffset + payloadSizeLen

	/* Predefined statuses of the TCP transport */
	tcpTransportStatusAccepting = "transport:tcp:status:accepting"

	/* Predefined message types */
	// INVALID represents an invalid tyoe
	INVALID = "INTERNAL:INVALID"
	// TERMMSG is the type for a termination message
	TERMMSG = "INTERNAL:TERMINA"
	// CONNREQ is the type for a connection request message
	CONNREQ = "INTERNAL:CONNECT"
	// CONNRED is the type for a connection redirection message to another port
	CONNRED = "INTERNAL:REDIREC"
	// CONNACK is the type for a connection ack message
	CONNACK = "INTERNAL:CONNACK"
	// DATA is the type for a data message
	DATAMSG = "INTERNAL:DATAMSG"
)

// TCPTransportCfg is the structure capturing the configuration of a
// TCP transport
type TCPTransportCfg struct {
	// Interface is the address used to accept connections, e.g., '172.0.0.1'
	Interface string

	// PortLow is the lowest port number that can be used to establish connections
	PortLow uint16

	// PortHigh is the high port number that can be used to establish connections
	PortHigh uint16

	// Accept specifies whether the TCP transport accepts incoming connections
	Accept bool

	// MaxRetry is the maximum of retries when trying to connect
	MaxRetry int

	// MTU is the requested MTU size
	MTU int64
}

// TCPTransport is the structure representing a given instantiation of a TCP transport
type TCPTransport struct {
	// Cfg is the configuration of the TCP transport
	Cfg *TCPTransportCfg

	// Status is the current status of the transport`
	Status string

	// Conn is a pointer to the underlying TCP connection
	Conn net.Conn

	receiverEPs []string
	remoteEPs   []string

	// RX pool
	RxPool pool.Pool
	// TX pool
	TxPool pool.Pool
	// sendQueue
	sendQueue chan []byte
	// RecvQueue is the receive queue, accessed by the endpoint to generate RX events
	RecvQueue chan []byte
}

type TCPHeader struct {
	// MsgType is the type of the message (specific to the transport)
	MsgType string

	// Src is the unique local endpoint ID of the sender
	Src string

	// Dst is the unique remote endpoint ID of the receiver
	Dst string
}

type tcpMsg struct {
	hdr     TCPHeader
	payload []byte
}

func (tpt *TCPTransport) sendTX(tx []byte) error {
	if tpt == nil {
		return fmt.Errorf("undefined transport")
	}

	s, err := tpt.Conn.Write(tx)
	if err != nil || s != len(tx) {
		return fmt.Errorf("failed to send TX")
	}

	return nil
}

func setHeader(tx []byte, hdr TCPHeader) {
	// The loops are separate to give a chance to the compiler to do some optimization
	for i := 0; i < len(hdr.MsgType); i++ {
		tx[msgTypeOffset+i] = byte(hdr.MsgType[i])
	}

	for i := 0; i < len(hdr.Src); i++ {
		tx[srcOffset+i] = byte(hdr.Src[i])
	}

	for i := 0; i < len(hdr.Dst); i++ {
		tx[dstOffset+i] = byte(hdr.Dst[i])
	}
}

func getHeader(conn net.Conn, rx []byte) error {
	addr := conn.LocalAddr()
	log.Printf("Receiving header from %s:%s\n", addr.Network(), addr.String())

	// Get the message's type
	n, err := conn.Read(rx[msgTypeOffset:msgTypeLen])
	if err != nil || n != msgTypeLen {
		return fmt.Errorf("unable to get TCP message header")
	}
	log.Printf("(%s) Msg type: %s", addr.String(), string(rx[msgTypeOffset:msgTypeLen]))

	// Get the src endpoint ID
	n, err = conn.Read(rx[srcOffset : srcOffset+srcLen])
	if err != nil || n != srcLen {
		return fmt.Errorf("unable to get TCP message header")
	}
	log.Printf("(%s) Msg recv'd from '%s'", addr.String(), string(rx[srcOffset:srcOffset+srcLen]))

	// Get the dst endpoint ID
	n, err = conn.Read(rx[dstOffset : dstOffset+dstLen])
	if err != nil || n != dstLen {
		return fmt.Errorf("unable to get TCP message header")
	}
	log.Printf("(%s) Msg to '%s'", addr.String(), string(rx[dstOffset:dstOffset+dstLen]))

	return nil
}

func getPayloadWithSize(conn net.Conn, payloadSize uint64, rx []byte) error {
	if payloadSize == 0 {
		return nil
	}

	readSize, err := conn.Read(rx[payloadOffset:])
	if readSize != int(payloadSize) || err != nil {
		return fmt.Errorf("unable to receive payload")
	}
	return nil
}

func setPayload(tx []byte, payload []byte) {
	// Unfortunately, Go can be a pain: there is no way to know how many bytes are used
	// for the length of the payload so we need to store the length of the size, which fits
	// into a single byte
	n := binary.PutUvarint(tx[payloadSizeOffset:payloadOffset], uint64(len(payload)))
	tx[sizeOfSizeOffset] = uint8(n)
	for i := 0; i < len(payload); i++ {
		tx[payloadOffset+i] = payload[i]
	}
}

// Read the payload from the socket and put it in a RX buffer
func getPayload(conn net.Conn, rx []byte) error {
	// Unfortunately, Go can be a pain: there is no way to know how many bytes are used
	// for the length of the payload so we need to store the length of the size, which fits
	// into a single byte
	sizeOfSize := int(rx[sizeOfSizeOffset])
	payloadSize, n := binary.Uvarint(rx[payloadSizeOffset : payloadSizeOffset+sizeOfSize])
	if n != sizeOfSize {
		return fmt.Errorf("failed to read the payload size from TCP msg")
	}
	return getPayloadWithSize(conn, payloadSize, rx)
}

func (tpt *TCPTransport) SendMsg(hdr TCPHeader, payload []byte) error {
	tx := tpt.TxPool.Get()
	if tx == nil {
		return fmt.Errorf("unable to get TX buffer")
	}

	setHeader(tx, hdr)
	setPayload(tx, payload)
	tpt.sendQueue <- tx

	return nil
}

func recvMsg(conn net.Conn, rx []byte) (int, error) {
	n, err := conn.Read(rx)
	if n == 0 {
		log.Println("[tcp:recvMsg] Conection closed, terminating...")
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to received data")
	}
	log.Printf("Successfully received %d bytes\n", n)

	return n, nil
}

func sendConnAck(tcp *TCPTransport, src string, dst string) error {
	// Get an TX
	tx := tcp.TxPool.Get()
	if tx == nil {
		return fmt.Errorf("unable to get TX")
	}

	// Set the TX
	hdr := TCPHeader{
		MsgType: CONNACK,
		Src:     src,
		Dst:     dst,
	}

	// No payload
	setHeader(tx, hdr)
	setPayload(tx, nil)

	// Add the send queue
	tcp.sendQueue <- tx

	return nil
}

func handleConnReq(tcp *TCPTransport, rx []byte) {
	// if more than one port are to the used and the port is not the lower one,
	// accept the connection
	src := tcp.receiverEPs[0] // todo: handle multiple endpoints per transport
	dst := string(rx[srcOffset : srcOffset+srcLen])
	log.Printf("Recv'd connection request from %s\n", dst)
	log.Println("Sending connection ack")
	sendConnAck(tcp, src, dst)

	// todo: if all the ports available are used: multiplex
}

func handleTermMsg() bool {
	// In case of multiplexing, we need to go through the data we track to
	// know if we need to stop the receive thread. Otherwise, we just stop the
	// receive thread
	return true
}

func (t *TCPTransport) ExtractPayload(rx []byte) ([]byte, error) {
	sizeOfSize := int(rx[sizeOfSizeOffset])
	payloadSize, n := binary.Uvarint(rx[payloadSizeOffset : payloadSizeOffset+sizeOfSize])
	if n != sizeOfSize {
		return nil, fmt.Errorf("failed to read the payload size from TCP msg")
	}
	return rx[payloadOffset : payloadOffset+payloadSize], nil
}

func (t *TCPTransport) ExtractDest(rx []byte) string {
	return string(rx[dstOffset : dstOffset+dstLen])
}

// GetSrcFromRX returns the subset of the RX storing the ID of the message's source.
// Note that the value is concidered invalid once the RX is returned; it is the
// responsability of the caller to make a copy as required.
func (tpt *TCPTransport) ExtractSrc(rx []byte) []byte {
	return rx[srcOffset : srcOffset+srcLen]
}

func handleConnRedirect(tcp *TCPTransport, rx []byte) error {
	// During a connection attempt, we are being redirected to another port
	src := string(rx[srcOffset : srcOffset+srcLen])
	payload, err := tcp.ExtractPayload(rx)
	if err != nil {
		return fmt.Errorf("unable to extract payload from RX: %w", err)
	}
	port, err := strconv.Atoi(string(payload))
	if err != nil {
		return fmt.Errorf("failed to extract port from payload: %w", err)
	}
	_, err = tcp.ConnectToPort(src, uint16(port))
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	return nil
}

func handleConnAck(tcp *TCPTransport, rx []byte) {
	// Connection succeeded, we get the remote endpoint ID, save it and return the RX
	remoteEPid := rx[dstOffset : dstOffset+dstLen]
	tcp.remoteEPs = append(tcp.remoteEPs, string(remoteEPid))
	err := tcp.RxPool.Return(rx)
	if err != nil {
		log.Println("[ERROR:tcp] unable to return RX")
	}
	log.Println("CONNACK successfully handled; connection fully established")
}

func recvThread(tcp *TCPTransport) {
	for {
		rx := tcp.RxPool.Get()
		if rx == nil {
			log.Println("[ERROR:tcp] unable to get RX buffer")
			return
		}

		n, err := recvMsg(tcp.Conn, rx)
		if n == 0 && err == nil {
			// Connection close, terminating
			log.Println("[tcp:recvThread] Terminating...")
			return
		}
		if err != nil {
			log.Printf("unable to receive data: %s", err)
		}

		msgType := string(rx[msgTypeOffset : msgTypeOffset+msgTypeLen])
		switch msgType {
		case DATAMSG:
			log.Println("DATAMSG recv'd")
			tcp.RecvQueue <- rx
			// The event thread at the endpoint level will get the RX and generate
			// an event so that the application can do a ep.GetRXEvent()
		case CONNREQ:
			log.Println("CONNREQ recv'd")
			handleConnReq(tcp, rx)
			err := tcp.RxPool.Return(rx)
			if err != nil {
				log.Println("unable to return RX buffer")
			}
		case TERMMSG:
			log.Println("TERMMSG recv'd")
			mustExit := handleTermMsg()
			err := tcp.RxPool.Return(rx)
			if err != nil {
				log.Println("unable to return RX buffer")
			}
			if mustExit {
				return
			}
		case CONNRED:
			log.Println("CONNRED recv'd")
			handleConnRedirect(tcp, rx)
			err := tcp.RxPool.Return(rx)
			if err != nil {
				log.Println("unable to return RX buffer")
			}
		case CONNACK:
			log.Println("CONNACK recv'd")
			handleConnAck(tcp, rx)
			err := tcp.RxPool.Return(rx)
			if err != nil {
				log.Println("unable to return RX buffer")
			}
		default:
			log.Printf("[ERROR:tcp] messages of type %s are not yet supported", msgType)
			err := tcp.RxPool.Return(rx)
			if err != nil {
				log.Println("unable to return RX buffer")
			}
		}
	}
}

func sendThread(tcp *TCPTransport) {
	for {
		if tcp != nil && tcp.Conn != nil {
			addr := tcp.Conn.LocalAddr()

			tx := <-tcp.sendQueue
			if tx == nil {
				return
			}
			log.Printf("(%s) New TX to send...", addr.String())
			n, err := tcp.Conn.Write(tx)
			if n == 0 {
				// Connection is closed, terminating
				log.Println("[INFO:sendThread] Connection closed, terminating")
				return
			}
			if n != len(tx) {
				log.Printf("[ERROR:sendThread] %d were sent instead of %d", n, len(tx))
			}
			// at the moment, even if send() failed, we return the TX
			log.Printf("(%s) Send succeeded (%d bytes), returning TX...", addr.String(), n)
			err = tcp.TxPool.Return(tx)
			if err != nil {
				log.Println("[ERROR:sendThread] unable to return TX")
			}
		}
	}
}

// Init creates a new TCP transport based on a configuration
func (cfg *TCPTransportCfg) Init() *TCPTransport {
	var tcp TCPTransport
	tcp.Cfg = cfg
	if tcp.Cfg.MaxRetry == 0 {
		tcp.Cfg.MaxRetry = defaultTCPMaxRetry
	}

	tcp.TxPool = pool.Pool{
		ObjSize:    defaultMTU,
		NObj:       defaultNumTX,
		GrowFactor: 0,
		Erase:      true,
	}

	tcp.RxPool = pool.Pool{
		ObjSize:    defaultMTU,
		NObj:       defaultNumRX,
		GrowFactor: 0,
		Erase:      true,
	}
	if cfg.MTU != 0 {
		tcp.TxPool.ObjSize = cfg.MTU
		tcp.RxPool.ObjSize = cfg.MTU
	}

	tcp.TxPool.New()
	tcp.RxPool.New()

	tcp.sendQueue = make(chan []byte)
	tcp.RecvQueue = make(chan []byte)

	if cfg.Accept {
		serverID := util.GenerateID()
		tcp.receiverEPs = append(tcp.receiverEPs, serverID)
		err := tcp.Accept(serverID)
		if err != nil {
			log.Printf("[ERROR:tcp] unable to accept incoming connections: %s", err)
			return nil
		}
	}

	return &tcp
}

// Close closes the current connection associated to the transport
func (tpt *TCPTransport) Close() error {
	err := tpt.Conn.Close()
	if err != nil {
		return fmt.Errorf("unable to close TCP connection: %w", err)
	}

	return nil
}

// IsAcceptingConns checks the status of the transport and if incoming connections can be accepted
func (tpt *TCPTransport) IsAcceptingConns() bool {
	if tpt == nil {
		log.Println("[ERROR:tcp] undefined transport")
		return false
	}

	if tpt.Status == tcpTransportStatusAccepting {
		return true
	}
	return false
}

// Accept accepts an incoming TCP connection using a given TCP transport
func (tpt *TCPTransport) Accept(epID string) error {
	if tpt == nil || tpt.Cfg == nil {
		return fmt.Errorf("corrupted transport object")
	}

	if !tpt.Cfg.Accept {
		return fmt.Errorf("attempting to accept connections on a transport not setup for it")
	}
	tpt.Status = tcpTransportStatusAccepting
	tpt.receiverEPs = append(tpt.receiverEPs, epID)

	listener, err := net.Listen("tcp", tpt.Cfg.Interface+":"+strconv.Itoa(int(tpt.Cfg.PortLow)))
	if err != nil {
		return fmt.Errorf("listen failed while acception new TCP connection: %w", err)
	}

	for {
		tpt.Conn, err = listener.Accept()
		if err == nil {
			// Connection established
			break
		}
	}

	// Start the send thread
	go sendThread(tpt)

	// Make sure to establish the connection before we start the generic recv thread
	rx := tpt.RxPool.Get()
	if rx == nil {
		return fmt.Errorf("[ERROR:tcp] unable to get RX buffer")
	}
	_, err = recvMsg(tpt.Conn, rx)
	if err != nil {
		return fmt.Errorf("unable to receive data: %s", err)
	}
	msgType := string(rx[msgTypeOffset : msgTypeOffset+msgTypeLen])
	if msgType != CONNREQ {
		return fmt.Errorf("receive a %s message instead of CONNREQ", msgType)
	}
	handleConnReq(tpt, rx)
	err = tpt.RxPool.Return(rx)
	if err != nil {
		log.Println("unable to return RX buffer")
	}

	// Start the receive thread
	go recvThread(tpt)

	log.Println("New connection accepted...")

	return nil
}

func (tpt *TCPTransport) initHandshake(epID string) (string, error) {
	// Get an TX
	tx := tpt.TxPool.Get()
	if tx == nil {
		return "", fmt.Errorf("unable to get TX")
	}

	// Set the TX
	hdr := TCPHeader{
		MsgType: CONNREQ,
		Src:     epID,
	}

	// No payload
	setHeader(tx, hdr)
	setPayload(tx, nil)
	// Add the send queue
	log.Println("Queuing TX for handshake...")
	tpt.sendQueue <- tx

	// Wait for CONNACK
	rx := tpt.RxPool.Get()
	if rx == nil {
		return "", fmt.Errorf("[ERROR:tcp] unable to get RX buffer")
	}
	_, err := recvMsg(tpt.Conn, rx)
	if err != nil {
		return "", fmt.Errorf("unable to receive data: %s", err)
	}
	msgType := string(rx[msgTypeOffset : msgTypeOffset+msgTypeLen])
	if msgType != CONNACK {
		return "", fmt.Errorf("receive a %s message instead of CONNREQ", msgType)
	}

	var serverID []byte
	id := tpt.ExtractSrc(rx)
	copy(serverID, id)
	tpt.RxPool.Return(rx)

	log.Println("Handshake completed")

	return string(serverID), nil
}

func (tpt *TCPTransport) Connect(epID string) (string, error) {
	if tpt == nil || tpt.Cfg == nil {
		log.Println("[ERROR:tcp] corrupted transport object")
		return "", nil
	}
	log.Printf("Connecting to %s:%d\n", tpt.Cfg.Interface, tpt.Cfg.PortLow)
	return tpt.ConnectToPort(epID, tpt.Cfg.PortLow)
}

// Connect creates a connection using a given transport
func (tpt *TCPTransport) ConnectToPort(epID string, port uint16) (string, error) {
	var err error
	retry := 0
	ip := tpt.Cfg.Interface
Retry:
	tpt.Conn, err = net.Dial("tcp", ip+":"+strconv.Itoa(int(port)))
	if err != nil {
		if retry < tpt.Cfg.MaxRetry {
			retry++
			time.Sleep(time.Duration(retry) * time.Second)
			goto Retry
		}
		return "", nil
	}

	// Start the send thread
	go sendThread(tpt)

	log.Println("Connection succeeded, initiating handshake...")
	serverID, err := tpt.initHandshake(epID)
	if err != nil {
		return "", fmt.Errorf("[ERROR] unable to initiate connection handshake: %w", err)
	}

	// Start receive thread
	log.Println("Starting recv thread...")
	go recvThread(tpt)

	log.Println("Connect() completed")
	return serverID, nil
}

// GetMsgTypeFromRX is a helper function that parses a given RX buffer and returns the
// message type
func (tpt *TCPTransport) GetMsgTypeFromRX(rx []byte) string {
	return string(rx[msgTypeOffset : msgTypeOffset+msgTypeLen])
}

// GetPayloadFromRX is a helper function that parses a given RX buffer and returns the
// payload
func (tpt *TCPTransport) GetPayloadFromRX(rx []byte) []byte {
	sizeOfSize := int(rx[sizeOfSizeOffset])
	payloadSize, n := binary.Uvarint(rx[payloadSizeOffset : payloadSizeOffset+sizeOfSize])
	if n != sizeOfSize {
		log.Println("[ERROR] failed to read the payload size from TCP msg")
		return nil
	}
	if payloadSize == 0 {
		return nil
	}
	return rx[msgTypeOffset : msgTypeOffset+payloadSize]
}

func (tpt *TCPTransport) Fini() {
	// todo: implementation here
}

func (tpt *TCPTransport) SendTermMsg(src string, dst string) error {
	hdr := TCPHeader{
		MsgType: TERMMSG,
		Src:     src,
		Dst:     dst,
	}
	err := tpt.SendMsg(hdr, nil)
	if err != nil {
		return fmt.Errorf("unable to send termination message: %s", err)
	}

	return nil
}
