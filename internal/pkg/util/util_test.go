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

func TestSameNetwork(t *testing.T) {
	tests := []struct {
		name            string
		id1             string
		id2             string
		successExpected bool
	}{
		{
			name:            "same IP",
			id1:             "127.0.0.1",
			id2:             "127.0.0.1/8",
			successExpected: true,
		},
		{
			name:            "same local network",
			id1:             "192.168.1.2",
			id2:             "192.168.1.3/8",
			successExpected: true,
		},
		{
			name:            "same networks 1",
			id1:             "10.0.1.4",
			id2:             "10.0.0.4/16",
			successExpected: true,
		},
		{
			name:            "same networks 2",
			id1:             "10.0.1.4",
			id2:             "10.1.0.4/24",
			successExpected: true,
		},
		{
			name:            "different networks",
			id1:             "192.168.1.1",
			id2:             "10.0.0.4/24",
			successExpected: false,
		},
		{
			name:            "different networks 1",
			id1:             "127.0.0.1",
			id2:             "127.0.1.1/8",
			successExpected: false,
		},
		{
			name:            "different networks 2",
			id1:             "10.1.0.4",
			id2:             "10.0.0.4/16",
			successExpected: false,
		},
		{
			name:            "different networks 3",
			id1:             "11.0.1.4",
			id2:             "10.0.1.4/24",
			successExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret := SameNetwork(tt.id1, tt.id2)
			if ret != tt.successExpected {
				t.Fatalf("%s case: %s and %s expected to return %v but returned %v", tt.name, tt.id1, tt.id2, tt.successExpected, ret)
			}
		})
	}
}
