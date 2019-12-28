package util

import (
	"log"
	"testing"
)

func TestGetLocalInferfaces(t *testing.T) {
	ifaces, err := GetLocalInferfaces()
	if err != nil {
		t.Fatal("GetLocalInferfaces() failed")
	}
	if len(ifaces) == 0 {
		t.Fatal("could not detect any network interface")
	}

	for _, iface := range ifaces {
		log.Printf("Interface %s: %s\n", iface.Name, iface.Addr)
	}
}
