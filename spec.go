package main

const (
	// ContainerImage is the Docker image used for all nodes.
	ContainerImage = "ghcr.io/zinrai/docker-debian-bird2:debian-trixie"
)

// Spec represents the tinet specification.
type Spec struct {
	Nodes       []Node       `yaml:"nodes"`
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
