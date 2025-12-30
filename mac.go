package main

import (
	"fmt"
	"net"
)

// GenerateMAC generates a locally administered MAC address.
// Format: 02:XX:XX:XX:XX:XX (U/L bit set for locally administered)
func GenerateMAC(linkID uint32) net.HardwareAddr {
	return net.HardwareAddr{
		0x02, // Locally administered (U/L bit = 1)
		byte(linkID >> 24),
		byte(linkID >> 16),
		byte(linkID >> 8),
		byte(linkID),
		0x00,
	}
}

// MACToLLA converts a MAC address to IPv6 link-local address using EUI-64.
// RFC 4291 Section 2.5.1
func MACToLLA(mac net.HardwareAddr) net.IP {
	if len(mac) != 6 {
		return nil
	}

	// EUI-64: insert FF:FE in the middle, flip U/L bit
	eui64 := make([]byte, 8)
	eui64[0] = mac[0] ^ 0x02 // Flip U/L bit
	eui64[1] = mac[1]
	eui64[2] = mac[2]
	eui64[3] = 0xFF
	eui64[4] = 0xFE
	eui64[5] = mac[3]
	eui64[6] = mac[4]
	eui64[7] = mac[5]

	// Construct fe80:: + EUI-64
	ip := make(net.IP, 16)
	ip[0] = 0xfe
	ip[1] = 0x80
	// ip[2:8] are zero (link-local prefix is fe80::/64)
	copy(ip[8:], eui64)

	return ip
}

// FormatLLAWithInterface formats LLA with interface scope for BIRD.
// Example: fe80::ff:fe00:1%eth0
func FormatLLAWithInterface(ip net.IP, iface string) string {
	return fmt.Sprintf("%s%%%s", ip, iface)
}
