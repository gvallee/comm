package util

import (
	"fmt"
	"math/rand"
	"net"
	"time"
)

// NetIface represents a network interface
type NetIface struct {
	Name string
	Addr string
}

// GetLocalInferfaces returns all the local network interfaces that can be detected
func GetLocalInferfaces() ([]NetIface, error) {
	var interfaces []NetIface
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("unable to get list of interfaces: %w", err)
	}

	// We select only interfaces for which we can get an IP
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			switch addr.Network() {
			case "ip+net":
				netiface := NetIface{
					Name: iface.Name,
					Addr: addr.String(),
				}
				interfaces = append(interfaces, netiface)
			}
		}
	}

	return interfaces, nil
}

// GenerateID creates a 256-character long random ID
func GenerateID() string {
	// generate a 256 random character string
	rand.Seed(time.Now().UnixNano())
	digits := "0123456789"
	all := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" + digits
	length := 256
	buf := make([]byte, length)
	buf[0] = digits[rand.Intn(len(digits))]
	for i := 2; i < length; i++ {
		buf[i] = all[rand.Intn(len(all))]
	}
	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})

	return string(buf)
}
