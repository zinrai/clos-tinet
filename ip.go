package main

import "fmt"

const (
	// AnycastAddress is the anycast address advertised by all servers.
	AnycastAddress = "10.100.0.1"
)

// SpineRouterID returns the router ID for a spine.
func SpineRouterID(index int) string {
	return fmt.Sprintf("10.255.0.%d", index+1)
}

// LeafRouterID returns the router ID for a leaf.
func LeafRouterID(pairIndex, leafNum int) string {
	return fmt.Sprintf("10.255.1.%d", pairIndex*2+leafNum)
}

// ToRRouterID returns the router ID for a ToR.
// Supports up to 510 ToRs (10.255.2.1 - 10.255.3.254).
func ToRRouterID(index int) string {
	if index < 255 {
		return fmt.Sprintf("10.255.2.%d", index+1)
	}
	return fmt.Sprintf("10.255.3.%d", index-254)
}

// BorderLeafRouterID returns the router ID for a border leaf.
func BorderLeafRouterID(index int) string {
	return fmt.Sprintf("10.255.254.%d", index+1)
}

// RouterRouterID returns the router ID for a router.
func RouterRouterID(index int) string {
	return fmt.Sprintf("10.255.255.%d", index+1)
}

// ServerRouterID returns the router ID for a server.
// Supports up to 65534 servers (10.0.0.1 - 10.0.255.254).
func ServerRouterID(index int) string {
	high := index / 256
	low := index%256 + 1
	return fmt.Sprintf("10.0.%d.%d", high, low)
}
