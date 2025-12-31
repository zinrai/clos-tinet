package main

import (
	"fmt"
)

// Topology builds a Clos network topology.
type Topology struct {
	config      Config
	templates   *Templates
	nodeConfigs []NodeConfig
	interfaces  map[string][]Interface
	birdConfigs map[string]string
	linkID      uint32                         // Link ID counter for MAC generation
	macCmds     map[string][]string            // MAC setting commands per node
	peerLLAs    map[string]map[string]peerInfo // node -> interface -> peer info
}

// peerInfo holds peer information for a link.
type peerInfo struct {
	PeerLLA  string
	PeerASN  int
	LocalLLA string
}

// NewTopology creates a new topology builder.
func NewTopology(cfg Config, templates *Templates) *Topology {
	return &Topology{
		config:      cfg,
		templates:   templates,
		interfaces:  make(map[string][]Interface),
		birdConfigs: make(map[string]string),
		macCmds:     make(map[string][]string),
		peerLLAs:    make(map[string]map[string]peerInfo),
	}
}

// Build generates the complete tinet specification.
func (t *Topology) Build() (Spec, error) {
	if err := t.buildSpines(); err != nil {
		return Spec{}, err
	}
	if err := t.buildLeafs(); err != nil {
		return Spec{}, err
	}
	if err := t.buildBorderLeafs(); err != nil {
		return Spec{}, err
	}
	if err := t.buildToRs(); err != nil {
		return Spec{}, err
	}
	if err := t.buildServers(); err != nil {
		return Spec{}, err
	}
	if err := t.buildRouters(); err != nil {
		return Spec{}, err
	}

	spec := Spec{
		Nodes:       t.buildNodes(),
		NodeConfigs: t.nodeConfigs,
	}

	// Add switches if external network is enabled
	if t.config.ExternalNetwork {
		spec.Switches = []Switch{
			{
				Name:       ExternalBridgeName,
				Interfaces: []Interface{},
			},
		}
	}

	return spec, nil
}

// GetBirdConfigs returns the generated BIRD configuration files.
func (t *Topology) GetBirdConfigs() map[string]string {
	return t.birdConfigs
}

// addInterface adds an interface definition to a node.
func (t *Topology) addInterface(nodeName, ifName, targetNode, targetIf string) {
	t.interfaces[nodeName] = append(t.interfaces[nodeName], Interface{
		Name: ifName,
		Type: "direct",
		Args: fmt.Sprintf("%s#%s", targetNode, targetIf),
	})
}

// addBridgeInterface adds a bridge interface definition to a node.
func (t *Topology) addBridgeInterface(nodeName, ifName, bridgeName string) {
	t.interfaces[nodeName] = append(t.interfaces[nodeName], Interface{
		Name: ifName,
		Type: "bridge",
		Args: bridgeName,
	})
}

// addLink creates a link between two nodes and sets up MAC/LLA mappings.
// Returns the peer LLAs for each side (what node1 sees, what node2 sees).
func (t *Topology) addLink(
	node1, if1 string, asn1 int,
	node2, if2 string, asn2 int,
) {
	// Generate MAC addresses for both ends
	mac1 := GenerateMAC(t.linkID)
	mac2 := GenerateMAC(t.linkID + 1)
	t.linkID += 2

	// Calculate LLAs
	lla1 := MACToLLA(mac1)
	lla2 := MACToLLA(mac2)

	// Store MAC and LLA setting commands
	t.macCmds[node1] = append(t.macCmds[node1],
		fmt.Sprintf("ip link set dev %s address %s", if1, mac1),
		fmt.Sprintf("ip -6 addr add %s/64 dev %s", lla1, if1),
	)
	t.macCmds[node2] = append(t.macCmds[node2],
		fmt.Sprintf("ip link set dev %s address %s", if2, mac2),
		fmt.Sprintf("ip -6 addr add %s/64 dev %s", lla2, if2),
	)

	// Add interface (one side only, tinet auto-generates reverse)
	t.addInterface(node1, if1, node2, if2)

	// Store peer LLA info (what each node sees on this interface)
	if t.peerLLAs[node1] == nil {
		t.peerLLAs[node1] = make(map[string]peerInfo)
	}
	if t.peerLLAs[node2] == nil {
		t.peerLLAs[node2] = make(map[string]peerInfo)
	}

	// node1 sees lla2 via if1, node2 sees lla1 via if2
	t.peerLLAs[node1][if1] = peerInfo{
		PeerLLA:  FormatLLAWithInterface(lla2, if1),
		PeerASN:  asn2,
		LocalLLA: lla1.String(),
	}
	t.peerLLAs[node2][if2] = peerInfo{
		PeerLLA:  FormatLLAWithInterface(lla1, if2),
		PeerASN:  asn1,
		LocalLLA: lla2.String(),
	}
}

// getPeerInfo returns the peer LLA, peer ASN, and local LLA for a given node and interface.
func (t *Topology) getPeerInfo(nodeName, ifName string) (peerLLA string, peerASN int, localLLA string) {
	if nodeInfo, ok := t.peerLLAs[nodeName]; ok {
		if info, ok := nodeInfo[ifName]; ok {
			return info.PeerLLA, info.PeerASN, info.LocalLLA
		}
	}
	return "", 0, ""
}

func (t *Topology) buildNodes() []Node {
	var nodes []Node
	for _, nc := range t.nodeConfigs {
		nodes = append(nodes, Node{
			Name:       nc.Name,
			Image:      ContainerImage,
			Interfaces: t.interfaces[nc.Name],
		})
	}
	return nodes
}

func (t *Topology) buildSpines() error {
	for i := 0; i < t.config.NumSpines; i++ {
		name := fmt.Sprintf("spine%d", i)
		routerID := SpineRouterID(i)

		// Connect to Leafs
		for pairIdx := 0; pairIdx < t.config.NumLeafPairs; pairIdx++ {
			leafASN := LeafASN(pairIdx)
			for leafNum := 1; leafNum <= 2; leafNum++ {
				leafName := fmt.Sprintf("leaf%d-as%d", leafNum, leafASN)
				myIf := fmt.Sprintf("lf%d", pairIdx*2+(leafNum-1))
				peerIf := fmt.Sprintf("sp%d", i)

				t.addLink(name, myIf, ASNSpine, leafName, peerIf, leafASN)
			}
		}

		// Connect to Border Leafs
		for blIdx := 0; blIdx < t.config.NumBorderLeafs; blIdx++ {
			myIf := fmt.Sprintf("bl%d", blIdx)
			peerIf := fmt.Sprintf("sp%d", i)

			t.addLink(name, myIf, ASNSpine, fmt.Sprintf("bl%d", blIdx), peerIf, ASNBorderLeaf)
		}

		// Build neighbors
		var neighbors []Neighbor

		// Leaf neighbors
		for pairIdx := 0; pairIdx < t.config.NumLeafPairs; pairIdx++ {
			leafASN := LeafASN(pairIdx)
			for leafNum := 1; leafNum <= 2; leafNum++ {
				myIf := fmt.Sprintf("lf%d", pairIdx*2+(leafNum-1))
				peerLLA, peerASN, localLLA := t.getPeerInfo(name, myIf)

				neighbors = append(neighbors, Neighbor{
					Name:         fmt.Sprintf("leaf%d_as%d", leafNum, leafASN),
					Interface:    myIf,
					PeerASN:      peerASN,
					PeerLLA:      peerLLA,
					LocalLLA:     localLLA,
					ImportFilter: "spine_import",
					ExportFilter: "spine_export",
					MaxPrefix:    500,
				})
			}
		}

		// Border Leaf neighbors
		for blIdx := 0; blIdx < t.config.NumBorderLeafs; blIdx++ {
			myIf := fmt.Sprintf("bl%d", blIdx)
			peerLLA, peerASN, localLLA := t.getPeerInfo(name, myIf)

			neighbors = append(neighbors, Neighbor{
				Name:         fmt.Sprintf("bl%d", blIdx),
				Interface:    myIf,
				PeerASN:      peerASN,
				PeerLLA:      peerLLA,
				LocalLLA:     localLLA,
				ImportFilter: "spine_import",
				ExportFilter: "spine_export",
				MaxPrefix:    100,
			})
		}

		if err := t.addNodeConfig(name, routerID, "spine", ASNSpine, neighbors, false); err != nil {
			return err
		}
	}
	return nil
}

func (t *Topology) buildLeafs() error {
	for pairIdx := 0; pairIdx < t.config.NumLeafPairs; pairIdx++ {
		leafASN := LeafASN(pairIdx)

		for leafNum := 1; leafNum <= 2; leafNum++ {
			name := fmt.Sprintf("leaf%d-as%d", leafNum, leafASN)
			routerID := LeafRouterID(pairIdx, leafNum)

			// Connect to ToRs
			for torIdx := 0; torIdx < t.config.NumToRsPerLeafPair; torIdx++ {
				globalToRIdx := pairIdx*t.config.NumToRsPerLeafPair + torIdx
				torASN := ToRASN(globalToRIdx)
				torName := fmt.Sprintf("tor%d-as%d", globalToRIdx, torASN)
				myIf := fmt.Sprintf("tr%d", torIdx)
				peerIf := fmt.Sprintf("lf%d", leafNum-1)

				t.addLink(name, myIf, leafASN, torName, peerIf, torASN)
			}

			// Build neighbors
			var neighbors []Neighbor

			// Spine neighbors (peer info was set when spines were built)
			for spineIdx := 0; spineIdx < t.config.NumSpines; spineIdx++ {
				peerIf := fmt.Sprintf("sp%d", spineIdx)
				peerLLA, peerASN, localLLA := t.getPeerInfo(name, peerIf)

				neighbors = append(neighbors, Neighbor{
					Name:         fmt.Sprintf("spine%d", spineIdx),
					Interface:    peerIf,
					PeerASN:      peerASN,
					PeerLLA:      peerLLA,
					LocalLLA:     localLLA,
					ImportFilter: "leaf_import_from_spine",
					ExportFilter: "leaf_export_to_spine",
					MaxPrefix:    1000,
				})
			}

			// ToR neighbors
			for torIdx := 0; torIdx < t.config.NumToRsPerLeafPair; torIdx++ {
				globalToRIdx := pairIdx*t.config.NumToRsPerLeafPair + torIdx
				myIf := fmt.Sprintf("tr%d", torIdx)
				peerLLA, peerASN, localLLA := t.getPeerInfo(name, myIf)

				neighbors = append(neighbors, Neighbor{
					Name:         fmt.Sprintf("tor%d", globalToRIdx),
					Interface:    myIf,
					PeerASN:      peerASN,
					PeerLLA:      peerLLA,
					LocalLLA:     localLLA,
					ImportFilter: "leaf_import_from_tor",
					ExportFilter: "leaf_export_to_tor",
					MaxPrefix:    100,
				})
			}

			if err := t.addNodeConfig(name, routerID, "leaf", leafASN, neighbors, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Topology) buildBorderLeafs() error {
	for blIdx := 0; blIdx < t.config.NumBorderLeafs; blIdx++ {
		name := fmt.Sprintf("bl%d", blIdx)
		routerID := BorderLeafRouterID(blIdx)

		// Connect to Routers
		for rtIdx := 0; rtIdx < t.config.NumRouters; rtIdx++ {
			rtName := fmt.Sprintf("router%d", rtIdx)
			myIf := fmt.Sprintf("rt%d", rtIdx)
			peerIf := fmt.Sprintf("bl%d", blIdx)

			t.addLink(name, myIf, ASNBorderLeaf, rtName, peerIf, ASNRouter)
		}

		// Build neighbors
		var neighbors []Neighbor

		// Spine neighbors (peer info was set when spines were built)
		for spineIdx := 0; spineIdx < t.config.NumSpines; spineIdx++ {
			peerIf := fmt.Sprintf("sp%d", spineIdx)
			peerLLA, peerASN, localLLA := t.getPeerInfo(name, peerIf)

			neighbors = append(neighbors, Neighbor{
				Name:         fmt.Sprintf("spine%d", spineIdx),
				Interface:    peerIf,
				PeerASN:      peerASN,
				PeerLLA:      peerLLA,
				LocalLLA:     localLLA,
				ImportFilter: "bl_import_from_spine",
				ExportFilter: "bl_export_to_spine",
				MaxPrefix:    1000,
			})
		}

		// Router neighbors
		for rtIdx := 0; rtIdx < t.config.NumRouters; rtIdx++ {
			myIf := fmt.Sprintf("rt%d", rtIdx)
			peerLLA, peerASN, localLLA := t.getPeerInfo(name, myIf)

			neighbors = append(neighbors, Neighbor{
				Name:         fmt.Sprintf("router%d", rtIdx),
				Interface:    myIf,
				PeerASN:      peerASN,
				PeerLLA:      peerLLA,
				LocalLLA:     localLLA,
				ImportFilter: "bl_import_from_router",
				ExportFilter: "bl_export_to_router",
				MaxPrefix:    10,
			})
		}

		if err := t.addNodeConfig(name, routerID, "bl", ASNBorderLeaf, neighbors, false); err != nil {
			return err
		}
	}
	return nil
}

func (t *Topology) buildToRs() error {
	for pairIdx := 0; pairIdx < t.config.NumLeafPairs; pairIdx++ {
		for torIdx := 0; torIdx < t.config.NumToRsPerLeafPair; torIdx++ {
			globalToRIdx := pairIdx*t.config.NumToRsPerLeafPair + torIdx
			torASN := ToRASN(globalToRIdx)
			name := fmt.Sprintf("tor%d-as%d", globalToRIdx, torASN)
			routerID := ToRRouterID(globalToRIdx)

			// Connect to Servers
			for srvIdx := 0; srvIdx < t.config.NumServersPerToR; srvIdx++ {
				globalSrvIdx := globalToRIdx*t.config.NumServersPerToR + srvIdx
				srvASN := ServerASN(globalSrvIdx)
				srvName := fmt.Sprintf("server%d-as%d", globalSrvIdx, srvASN)
				myIf := fmt.Sprintf("sv%d", srvIdx)
				peerIf := "tr0"

				t.addLink(name, myIf, torASN, srvName, peerIf, srvASN)
			}

			// Build neighbors
			var neighbors []Neighbor

			// Leaf neighbors (peer info was set when leafs were built)
			for leafNum := 1; leafNum <= 2; leafNum++ {
				peerIf := fmt.Sprintf("lf%d", leafNum-1)
				peerLLA, peerASN, localLLA := t.getPeerInfo(name, peerIf)

				neighbors = append(neighbors, Neighbor{
					Name:         fmt.Sprintf("leaf%d", leafNum),
					Interface:    peerIf,
					PeerASN:      peerASN,
					PeerLLA:      peerLLA,
					LocalLLA:     localLLA,
					ImportFilter: "tor_import_from_leaf",
					ExportFilter: "tor_export_to_leaf",
					MaxPrefix:    500,
				})
			}

			// Server neighbors
			for srvIdx := 0; srvIdx < t.config.NumServersPerToR; srvIdx++ {
				globalSrvIdx := globalToRIdx*t.config.NumServersPerToR + srvIdx
				myIf := fmt.Sprintf("sv%d", srvIdx)
				peerLLA, peerASN, localLLA := t.getPeerInfo(name, myIf)

				neighbors = append(neighbors, Neighbor{
					Name:         fmt.Sprintf("server%d", globalSrvIdx),
					Interface:    myIf,
					PeerASN:      peerASN,
					PeerLLA:      peerLLA,
					LocalLLA:     localLLA,
					ImportFilter: "tor_import_from_server",
					ExportFilter: "tor_export_to_server",
					MaxPrefix:    10,
				})
			}

			if err := t.addNodeConfig(name, routerID, "tor", torASN, neighbors, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Topology) buildServers() error {
	serverNum := 0
	for pairIdx := 0; pairIdx < t.config.NumLeafPairs; pairIdx++ {
		for torIdx := 0; torIdx < t.config.NumToRsPerLeafPair; torIdx++ {
			globalToRIdx := pairIdx*t.config.NumToRsPerLeafPair + torIdx

			for srvIdx := 0; srvIdx < t.config.NumServersPerToR; srvIdx++ {
				serverASN := ServerASN(serverNum)
				name := fmt.Sprintf("server%d-as%d", serverNum, serverASN)
				routerID := ServerRouterID(serverNum)

				// ToR neighbor (peer info was set when ToRs were built)
				peerIf := "tr0"
				peerLLA, peerASN, localLLA := t.getPeerInfo(name, peerIf)

				neighbors := []Neighbor{{
					Name:         fmt.Sprintf("tor%d", globalToRIdx),
					Interface:    peerIf,
					PeerASN:      peerASN,
					PeerLLA:      peerLLA,
					LocalLLA:     localLLA,
					ImportFilter: "server_import",
					ExportFilter: "server_export",
					MaxPrefix:    100,
				}}

				if err := t.addNodeConfig(name, routerID, "server", serverASN, neighbors, true); err != nil {
					return err
				}
				serverNum++
			}
		}
	}
	return nil
}

func (t *Topology) buildRouters() error {
	for rtIdx := 0; rtIdx < t.config.NumRouters; rtIdx++ {
		name := fmt.Sprintf("router%d", rtIdx)
		routerID := RouterRouterID(rtIdx)
		var neighbors []Neighbor

		// Border Leaf neighbors (peer info was set when BLs were built)
		for blIdx := 0; blIdx < t.config.NumBorderLeafs; blIdx++ {
			peerIf := fmt.Sprintf("bl%d", blIdx)
			peerLLA, peerASN, localLLA := t.getPeerInfo(name, peerIf)

			neighbors = append(neighbors, Neighbor{
				Name:         fmt.Sprintf("bl%d", blIdx),
				Interface:    peerIf,
				PeerASN:      peerASN,
				PeerLLA:      peerLLA,
				LocalLLA:     localLLA,
				ImportFilter: "router_import",
				ExportFilter: "router_export",
				MaxPrefix:    1000,
			})
		}

		// Add external network interface if enabled
		if t.config.ExternalNetwork {
			t.addBridgeInterface(name, "eth0", ExternalBridgeName)
		}

		if err := t.addRouterNodeConfig(name, routerID, ASNRouter, neighbors, rtIdx); err != nil {
			return err
		}
	}
	return nil
}

// addRouterNodeConfig adds a router node configuration with optional external network settings.
func (t *Topology) addRouterNodeConfig(name, routerID string, asn int, neighbors []Neighbor, routerIndex int) error {
	// Generate BIRD config using template
	data := TemplateData{
		RouterID:  routerID,
		ASN:       asn,
		Neighbors: neighbors,
	}

	birdConf, err := t.templates.Render("router", data)
	if err != nil {
		return fmt.Errorf("failed to render template for %s: %w", name, err)
	}

	t.birdConfigs[name] = birdConf

	cmds := []Command{
		{Cmd: fmt.Sprintf("ip addr add %s/32 dev lo", routerID)},
	}

	// Add MAC setting commands
	for _, macCmd := range t.macCmds[name] {
		cmds = append(cmds, Command{Cmd: macCmd})
	}

	cmds = append(cmds,
		Command{Cmd: "sysctl -w net.ipv4.ip_forward=1"},
		Command{Cmd: "sysctl -w net.ipv6.conf.all.forwarding=1"},
		Command{Cmd: fmt.Sprintf("cp /tinet/%s.conf /etc/bird/bird.conf", name)},
		Command{Cmd: "mkdir -p /run/bird"},
		Command{Cmd: "bird -c /etc/bird/bird.conf"},
	)

	// Add external network configuration if enabled
	if t.config.ExternalNetwork {
		externalIP := ExternalRouterIP(routerIndex)
		cmds = append(cmds,
			Command{Cmd: fmt.Sprintf("ip addr add %s/24 dev eth0", externalIP)},
			Command{Cmd: fmt.Sprintf("ip route add default via %s", ExternalNetworkGateway)},
			Command{Cmd: "iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE"},
		)
	}

	t.nodeConfigs = append(t.nodeConfigs, NodeConfig{Name: name, Cmds: cmds})
	return nil
}

func (t *Topology) addNodeConfig(name, routerID, role string, asn int, neighbors []Neighbor, isServer bool) error {
	// Generate BIRD config using template
	data := TemplateData{
		RouterID:  routerID,
		ASN:       asn,
		Neighbors: neighbors,
	}

	birdConf, err := t.templates.Render(role, data)
	if err != nil {
		return fmt.Errorf("failed to render template for %s: %w", name, err)
	}

	t.birdConfigs[name] = birdConf

	cmds := []Command{
		{Cmd: fmt.Sprintf("ip addr add %s/32 dev lo", routerID)},
	}

	if isServer {
		cmds = append(cmds, Command{Cmd: fmt.Sprintf("ip addr add %s/32 dev lo", AnycastAddress)})
	}

	// Add MAC setting commands
	for _, macCmd := range t.macCmds[name] {
		cmds = append(cmds, Command{Cmd: macCmd})
	}

	cmds = append(cmds,
		Command{Cmd: "sysctl -w net.ipv4.ip_forward=1"},
		Command{Cmd: "sysctl -w net.ipv6.conf.all.forwarding=1"},
		Command{Cmd: fmt.Sprintf("cp /tinet/%s.conf /etc/bird/bird.conf", name)},
		Command{Cmd: "mkdir -p /run/bird"},
		Command{Cmd: "bird -c /etc/bird/bird.conf"},
	)

	t.nodeConfigs = append(t.nodeConfigs, NodeConfig{Name: name, Cmds: cmds})
	return nil
}
