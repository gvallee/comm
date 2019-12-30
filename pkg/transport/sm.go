/*
 * Copyright (c) 2019 Geoffroy Vallee, All rights reserved
 * This software is licensed under a 3-clause BSD license. Please consult the
 * LICENSE.md file distributed with the sources of this project regarding your
 * rights to use or distribute this software.
 */

package transport

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	// SMTransport identifies the TCP transport
	SMTransportID = "SM"

	// For now, hardcode 512 message of 4096 bytes
	defaultBlockSize = 4096
	defaultNumBlocks = 512
)

type block struct {
	index int64
}

type mmapInfo struct {
	mmapFile  string
	fd        *os.File
	buffer    []byte
	size      int64
	blockSize int64
}

type smPeer struct {
	id        string
	recvQueue chan block
}

type SMTransportCfg struct {
	blockSize int64
	numBlocks int64
	peer1     string
	peer2     string
}

type SMTransport struct {
	cfg           SMTransportCfg
	curReadBlock  int64
	curWriteBlock int64
	availBlocks   chan block
	readBlocks    chan block
	peers         [2]smPeer
	mmapInfo      *mmapInfo
}

func (cfg *SMTransportCfg) Init() *SMTransport {
	var tpt SMTransport
	tpt.cfg = *cfg
	tpt.mmapInfo = cfg.createMMAP("anID")
	if tpt.mmapInfo == nil {
		log.Println("unable to create MMAP")
		return nil
	}
	if tpt.cfg.blockSize == 0 {
		tpt.cfg.blockSize = defaultBlockSize
	}
	if cfg.numBlocks == 0 {
		tpt.cfg.numBlocks = defaultNumBlocks
	}
	log.Printf("Initializing transport with %d blocks of size %d bytes\n", tpt.cfg.numBlocks, tpt.cfg.blockSize)
	tpt.mmapInfo.size = cfg.blockSize * cfg.numBlocks
	tpt.availBlocks = make(chan block, defaultNumBlocks)
	tpt.peers[0].recvQueue = make(chan block)
	tpt.peers[1].recvQueue = make(chan block)

	var i int64
	for i = 0; i < defaultNumBlocks; i++ {
		b := block{
			index: i,
		}
		tpt.availBlocks <- b
	}

	return &tpt
}

func (cfg *SMTransportCfg) createMMAP(epid string) *mmapInfo {
	var info mmapInfo
	var err error

	info.fd, err = ioutil.TempFile("", epid+"-")
	if err != nil {
		log.Printf("unable to create file for MMAP: %w", err)
		return nil
	}
	info.mmapFile = info.fd.Name()

	// initialize the file for mmap to the correct size
	// for that we do a single write at the desired size
	info.size = cfg.blockSize * cfg.numBlocks
	info.blockSize = cfg.blockSize
	data := make([]byte, info.size)
	_, err = info.fd.Write(data)
	if err != nil {
		log.Printf("Write failed: %s", err)
		return nil
	}

	fd := info.fd.Fd()
	info.buffer, err = unix.Mmap(int(fd), 0, int(info.size), syscall.PROT_READ|syscall.PROT_WRITE, unix.MAP_ANON|unix.MAP_SHARED)
	if err != nil {
		log.Printf("mmap() syscall failed: %s", err)
		return nil
	}

	return &info
}

func (info *mmapInfo) deleteMMAP() error {
	if info == nil {
		return nil
	}

	err := syscall.Munmap(info.buffer)
	if err != nil {
		return fmt.Errorf("unable to munmap buffer: %w", err)
	}

	info.fd.Close()
	err = os.RemoveAll(info.mmapFile)
	if err != nil {
		return fmt.Errorf("unable to delete %s: %w", info.mmapFile, err)
	}
	info.size = 0
	return nil
}

func (info *mmapInfo) doWriteBlock(idx int64, data []byte) error {
	err := info.writeAt(data, idx*info.blockSize)
	if err != nil {
		return fmt.Errorf("unable to actually write block of data")
	}
	return nil
}

// Write a slice of bytes into a mmap buffer
func (info *mmapInfo) write(data []byte) error {
	n := copy(info.buffer, data)
	if n != len(data) {
		return fmt.Errorf("wrote %d bytes instead of %d", n, len(info.buffer))
	}
	return nil
}

// Read the mmap buffer
func (info *mmapInfo) read() []byte {
	var data []byte
	n := copy(data, info.buffer)
	if n != len(info.buffer) {
		log.Printf("[ERROR:sm] Read %d bytes instead of %d", n, len(info.buffer))
		return nil
	}
	return data
}

// WriteAt writes a slice of data into the mmap buffer. It is the responsability
// of the caller to ensure that the mmap buffer is big enought.
func (info *mmapInfo) writeAt(data []byte, offset int64) error {
	n := copy(info.buffer[int(offset):], data)
	if n != len(data) {
		return fmt.Errorf("wrote %d bytes instead of %d", n, len(info.buffer))
	}
	return nil
}

// ReadAt reads data from an offset and a given length from a mmap buffer. It
// is the responsability of the caller to ensure the read can succeed.
func (info *mmapInfo) readAt(offset int64, len int64) []byte {
	data := make([]byte, info.blockSize)
	log.Printf("Reading %d bytes from offset %d\n", int(len), int(offset))
	n := copy(data, info.buffer[offset:offset+len])
	if n != int(len) {
		log.Printf("[ERROR:sm] Read %d bytes instead of %d", n, len)
		return nil
	}
	log.Printf("Read %d bytes", n)
	return data
}

func (tpt *SMTransport) Send(dst string, data []byte) error {
	// Get a block from the sender
	b := <-tpt.availBlocks

	// We own the block; write the data
	log.Printf("writing data to block: %d\n", b.index)
	err := tpt.mmapInfo.doWriteBlock(b.index, data)
	if err != nil {
		return fmt.Errorf("unable to write block: %w", err)
	}

	// Put block in read queue of the receiver
	if tpt.peers[0].id == dst {
		log.Printf("Moving block %d to read queue for peer 1", b.index)
		tpt.peers[0].recvQueue <- b
	} else {
		log.Printf("Moving block %d to read queue for peer 2", b.index)
		tpt.peers[1].recvQueue <- b
	}
	return nil
}

func (tpt *SMTransport) Recv(src string) []byte {
	var b block
	if tpt.peers[0].id == src {
		log.Printf("Getting message for peer 1 (%d msg pending)\n", len(tpt.peers[0].recvQueue))
		b = <-tpt.peers[0].recvQueue
	} else {
		log.Printf("Getting message for peer 2 (%d msg pending)\n", len(tpt.peers[0].recvQueue))
		b = <-tpt.peers[1].recvQueue
	}
	log.Printf("reading data from block: %d\n", b.index)
	data := tpt.mmapInfo.readAt(b.index*tpt.mmapInfo.blockSize, tpt.mmapInfo.blockSize)
	if data == nil {
		log.Printf("[ERROR:sm] unable to read block %d\n", b.index)
		return nil
	}
	log.Printf("returning block %d\n", b.index)
	tpt.availBlocks <- b
	return data
}

func (tpt *SMTransport) Disconnect(peerID string) error {
	// todo: implement me
	return nil
}

func (tpt *SMTransport) Fini() error {
	err := tpt.mmapInfo.deleteMMAP()
	if err != nil {
		return fmt.Errorf("failed to terminate MMAP: %w", err)
	}
	return nil
}
