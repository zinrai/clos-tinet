package main

import (
	"strings"
	"testing"
)

// testTemplates returns minimal templates for testing.
func testTemplates() *Templates {
	minimalTemplate := `router id {{ .RouterID }};
define LOCAL_AS = {{ .ASN }};
{{ range .Neighbors }}
protocol bgp {{ .Name }} {
        neighbor {{ .PeerLLA }} as {{ .PeerASN }};
        local {{ .LocalLLA }} as LOCAL_AS;
}
{{ end }}
`
	return &Templates{
		Spine:  minimalTemplate,
		Leaf:   minimalTemplate,
		BL:     minimalTemplate,
		ToR:    minimalTemplate,
		Server: minimalTemplate,
		Router: minimalTemplate,
	}
}

func TestRouterIDUniqueness(t *testing.T) {
	cfg := DefaultConfig()
	ids := make(map[string]string)

	// Spines
	for i := 0; i < cfg.NumSpines; i++ {
		id := SpineRouterID(i)
		name := "spine" + string(rune('0'+i))
		if existing, ok := ids[id]; ok {
			t.Errorf("Duplicate router ID %s: %s and %s", id, existing, name)
		}
		ids[id] = name
	}

	// Leafs
	for p := 0; p < cfg.NumLeafPairs; p++ {
		for l := 1; l <= 2; l++ {
			id := LeafRouterID(p, l)
			name := "leaf"
			if existing, ok := ids[id]; ok {
				t.Errorf("Duplicate router ID %s: %s and %s", id, existing, name)
			}
			ids[id] = name
		}
	}

	// ToRs
	for i := 0; i < cfg.TotalToRs(); i++ {
		id := ToRRouterID(i)
		name := "tor"
		if existing, ok := ids[id]; ok {
			t.Errorf("Duplicate router ID %s: %s and %s", id, existing, name)
		}
		ids[id] = name
	}

	// Border Leafs
	for i := 0; i < cfg.NumBorderLeafs; i++ {
		id := BorderLeafRouterID(i)
		name := "bl"
		if existing, ok := ids[id]; ok {
			t.Errorf("Duplicate router ID %s: %s and %s", id, existing, name)
		}
		ids[id] = name
	}

	// Routers
	for i := 0; i < cfg.NumRouters; i++ {
		id := RouterRouterID(i)
		name := "router"
		if existing, ok := ids[id]; ok {
			t.Errorf("Duplicate router ID %s: %s and %s", id, existing, name)
		}
		ids[id] = name
	}

	// Servers
	for i := 0; i < cfg.TotalServers(); i++ {
		id := ServerRouterID(i)
		name := "server"
		if existing, ok := ids[id]; ok {
			t.Errorf("Duplicate router ID %s: %s and %s", id, existing, name)
		}
		ids[id] = name
	}
}

func TestInterfaceConnections(t *testing.T) {
	cfg := DefaultConfig()
	topo := NewTopology(cfg, testTemplates())
	spec, err := topo.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Build connection map: node#interface -> target
	connections := make(map[string]map[string]string)
	for _, node := range spec.Nodes {
		connections[node.Name] = make(map[string]string)
		for _, iface := range node.Interfaces {
			if iface.Type == "direct" {
				connections[node.Name][iface.Name] = iface.Args
			}
		}
	}

	// Add reverse connections (tinet generates these automatically)
	for nodeName, ifaces := range connections {
		for ifName, target := range ifaces {
			parts := strings.Split(target, "#")
			if len(parts) == 2 {
				targetNode, targetIf := parts[0], parts[1]
				if connections[targetNode] == nil {
					connections[targetNode] = make(map[string]string)
				}
				reverseTarget := nodeName + "#" + ifName
				if existing, ok := connections[targetNode][targetIf]; ok {
					if existing != reverseTarget {
						t.Errorf("Interface conflict: %s#%s -> %s, but already -> %s",
							targetNode, targetIf, reverseTarget, existing)
					}
				} else {
					connections[targetNode][targetIf] = reverseTarget
				}
			}
		}
	}

	// Verify symmetry
	for nodeName, ifaces := range connections {
		for ifName, target := range ifaces {
			parts := strings.Split(target, "#")
			if len(parts) != 2 {
				t.Errorf("Invalid target format: %s", target)
				continue
			}
			targetNode, targetIf := parts[0], parts[1]

			// Check reverse connection exists
			if reverseTarget, ok := connections[targetNode][targetIf]; ok {
				expected := nodeName + "#" + ifName
				if reverseTarget != expected {
					t.Errorf("Asymmetric connection: %s#%s -> %s, but %s#%s -> %s (expected %s)",
						nodeName, ifName, target, targetNode, targetIf, reverseTarget, expected)
				}
			}
		}
	}
}

func TestNodeConfigUsesCP(t *testing.T) {
	cfg := DefaultConfig()
	topo := NewTopology(cfg, testTemplates())
	spec, err := topo.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	for _, nc := range spec.NodeConfigs {
		foundCP := false
		foundMkdir := false
		for _, cmd := range nc.Cmds {
			if strings.HasPrefix(cmd.Cmd, "cp /tinet/") && strings.HasSuffix(cmd.Cmd, ".conf /etc/bird/bird.conf") {
				foundCP = true
			}
			if cmd.Cmd == "mkdir -p /run/bird" {
				foundMkdir = true
			}
		}
		if !foundCP {
			t.Errorf("Node %s missing cp command for BIRD config", nc.Name)
		}
		if !foundMkdir {
			t.Errorf("Node %s missing mkdir command for /run/bird", nc.Name)
		}
	}
}

func TestMACCommandsGenerated(t *testing.T) {
	cfg := DefaultConfig()
	topo := NewTopology(cfg, testTemplates())
	spec, err := topo.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	for _, nc := range spec.NodeConfigs {
		// Router has no interfaces defined by itself (only by BL side)
		if strings.HasPrefix(nc.Name, "router") {
			continue
		}

		foundMAC := false
		for _, cmd := range nc.Cmds {
			if strings.Contains(cmd.Cmd, "ip link set dev") && strings.Contains(cmd.Cmd, "address 02:") {
				foundMAC = true
				break
			}
		}

		if !foundMAC {
			t.Errorf("Node %s missing MAC setting command", nc.Name)
		}
	}
}

func TestPeerLLAInBirdConfig(t *testing.T) {
	cfg := DefaultConfig()
	topo := NewTopology(cfg, testTemplates())
	_, err := topo.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	configs := topo.GetBirdConfigs()

	spine0Config, ok := configs["spine0"]
	if !ok {
		t.Fatal("Missing spine0 config")
	}

	if !strings.Contains(spine0Config, "neighbor fe80::") {
		t.Errorf("spine0 config missing LLA neighbor")
	}

	if !strings.Contains(spine0Config, "%") {
		t.Errorf("spine0 config missing interface scope")
	}

	if !strings.Contains(spine0Config, " as ") {
		t.Errorf("spine0 config missing 'as' clause")
	}
}

func TestMACUniqueness(t *testing.T) {
	cfg := DefaultConfig()
	topo := NewTopology(cfg, testTemplates())
	spec, err := topo.Build()
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	macs := make(map[string]string) // MAC -> "node:interface"

	for _, nc := range spec.NodeConfigs {
		for _, cmd := range nc.Cmds {
			if strings.Contains(cmd.Cmd, "ip link set dev") && strings.Contains(cmd.Cmd, "address") {
				parts := strings.Fields(cmd.Cmd)
				if len(parts) >= 6 {
					iface := parts[4]
					mac := parts[6]
					key := nc.Name + ":" + iface

					if existing, ok := macs[mac]; ok {
						t.Errorf("Duplicate MAC %s: %s and %s", mac, existing, key)
					}
					macs[mac] = key
				}
			}
		}
	}
}
