/*
 * Copyright (c) 2019 Geoffroy Vallee, All rights reserved
 * This software is licensed under a 3-clause BSD license. Please consult the
 * LICENSE.md file distributed with the sources of this project regarding your
 * rights to use or distribute this software.
 */

package comm

import (
	"fmt"
	"net"

	"github.com/gvallee/comm/pkg/transport"
)

// Connection is a structure representing a connection regardless of the
// underlying transport
type Connection struct {
	transport Transport

	// Save the actual connection
	tcpConn net.Conn
}

// Close closes a given connection
func (c *Connection) Close() error {
	if c == nil {
		return nil
	}

	switch c.transport.Priv.(type) {
	case *transport.TCPTransport:
		err := c.transport.TCP.Close()
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown transport type")
	}
	return nil
}
