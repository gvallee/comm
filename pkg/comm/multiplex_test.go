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
	"testing"
)

func doMultiplexServer(t *testing.T, nClient int) {
	log.Println("Hello, i am the server test")

	fmt.Println("All messages have been received")
}

func doMultiplexClient(t *testing.T, id string) {
	// Each client sends a single message and then terminates, once all
	// clients are terminated, the server is supposed to detect the
	// closed connection and terminates as well.
	log.Printf("Hello, i am a test client (%s)\n", id)
}

/*
func TestTCPMultiplexing(t *testing.T) {
	// One server using a single port; two clients
	go doServer(t)

	go doClient(t, "client1")
	doClient(t, "client2")
}
*/
