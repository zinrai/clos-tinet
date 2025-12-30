package main

import (
	"net"
	"testing"
)

func TestMACToLLA(t *testing.T) {
	tests := []struct {
		mac      string
		expected string
	}{
		// Locally administered MACs (U/L bit already set)
		{"02:00:00:00:00:00", "fe80::ff:fe00:0"},
		{"02:00:00:00:01:00", "fe80::ff:fe00:100"},
		{"02:00:00:01:00:00", "fe80::ff:fe01:0"},
		// Standard MAC (from RFC example style)
		{"00:12:7f:eb:6b:40", "fe80::212:7fff:feeb:6b40"},
	}

	for _, tt := range tests {
		mac, err := net.ParseMAC(tt.mac)
		if err != nil {
			t.Fatalf("Failed to parse MAC %s: %v", tt.mac, err)
		}
		got := MACToLLA(mac)
		if got.String() != tt.expected {
			t.Errorf("MACToLLA(%s) = %s, want %s", tt.mac, got, tt.expected)
		}
	}
}
