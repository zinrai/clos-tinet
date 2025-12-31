package main

import "fmt"

const (
	// ContainerImage is the Docker image used for all nodes.
	ContainerImage = "ghcr.io/zinrai/docker-debian-bird2:debian-trixie"

	// ExternalBridgeName is the OVS bridge name for external connectivity.
	ExternalBridgeName = "ext"

	// ExternalNetworkGateway is the gateway IP on the host side.
	ExternalNetworkGateway = "172.31.255.1"

	// ExternalNetworkPrefix is the subnet prefix for external network.
	ExternalNetworkPrefix = "172.31.255"
)

// Spec represents the tinet specification.
type Spec struct {
	Nodes       []Node       `yaml:"nodes"`
	Switches    []Switch     `yaml:"switches,omitempty"`
	NodeConfigs []NodeConfig `yaml:"node_configs"`
}

// Node represents a network node.
type Node struct {
	Name       string      `yaml:"name"`
	Image      string      `yaml:"image"`
	Interfaces []Interface `yaml:"interfaces"`
}

// Interface represents a network interface.
type Interface struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	Args string `yaml:"args"`
}

// Switch represents an OVS switch.
type Switch struct {
	Name       string      `yaml:"name"`
	Interfaces []Interface `yaml:"interfaces"`
}

// NodeConfig represents the configuration commands for a node.
type NodeConfig struct {
	Name string    `yaml:"name"`
	Cmds []Command `yaml:"cmds"`
}

// Command represents a shell command.
type Command struct {
	Cmd string `yaml:"cmd"`
}

// Neighbor represents a BGP neighbor.
type Neighbor struct {
	Name         string
	Interface    string
	PeerASN      int    // Peer's AS number
	PeerLLA      string // Peer's link-local address with interface scope (e.g., fe80::1%eth0)
	LocalLLA     string // Local link-local address (without interface scope)
	ImportFilter string
	ExportFilter string
	MaxPrefix    int
}

// ExternalRouterIP returns the external network IP for a router by index.
// router0 -> 172.31.255.2, router1 -> 172.31.255.3, etc.
func ExternalRouterIP(index int) string {
	return fmt.Sprintf("%s.%d", ExternalNetworkPrefix, index+2)
}
