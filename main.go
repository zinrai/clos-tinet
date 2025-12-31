package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

func main() {
	cfg := ParseFlags()

	// Validate external network options
	if cfg.ExternalNetwork && cfg.ExternalInterface == "" {
		fmt.Fprintf(os.Stderr, "Error: -external-interface is required when -external-network is enabled\n")
		os.Exit(1)
	}

	// Load templates
	templates, err := LoadTemplates(cfg.BirdTemplates)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading templates: %v\n", err)
		os.Exit(1)
	}

	// Build topology
	topo := NewTopology(cfg, templates)
	spec, err := topo.Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building topology: %v\n", err)
		os.Exit(1)
	}

	// Write BIRD config files
	if err := writeBirdConfigs(cfg.BirdConfigDir, topo.GetBirdConfigs()); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing BIRD configs: %v\n", err)
		os.Exit(1)
	}

	// Write spec to stdout
	if err := writeYAML(spec); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing YAML: %v\n", err)
		os.Exit(1)
	}

	// Print host setup commands if external network is enabled
	if cfg.ExternalNetwork {
		printHostSetupCommands(cfg)
	}
}

func writeBirdConfigs(dir string, configs map[string]string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	for name, config := range configs {
		path := filepath.Join(dir, name+".conf")
		if err := os.WriteFile(path, []byte(config), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}

	return nil
}

func writeYAML(spec Spec) error {
	data, err := yaml.MarshalWithOptions(spec, yaml.IndentSequence(true))
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func printHostSetupCommands(cfg Config) {
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "# Host setup commands for external network connectivity:")
	fmt.Fprintln(os.Stderr, "# Run these commands on the host after 'tinet up'")
	fmt.Fprintln(os.Stderr)

	// IP address on ext bridge
	fmt.Fprintf(os.Stderr, "sudo ip addr add %s/24 dev %s\n",
		ExternalNetworkGateway, ExternalBridgeName)

	// NAT rule
	fmt.Fprintf(os.Stderr, "sudo iptables -t nat -A POSTROUTING -s %s.0/24 -o %s -j MASQUERADE\n",
		ExternalNetworkPrefix, cfg.ExternalInterface)

	// FORWARD rules with source address
	fmt.Fprintf(os.Stderr, "sudo iptables -I FORWARD -s %s.0/24 -o %s -j ACCEPT\n",
		ExternalNetworkPrefix, cfg.ExternalInterface)
	fmt.Fprintf(os.Stderr, "sudo iptables -I FORWARD -d %s.0/24 -i %s -m state --state RELATED,ESTABLISHED -j ACCEPT\n",
		ExternalNetworkPrefix, cfg.ExternalInterface)

	// IP forwarding
	fmt.Fprintln(os.Stderr, "sudo sysctl -w net.ipv4.ip_forward=1")
}
