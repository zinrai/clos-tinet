package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
)

func main() {
	cfg := ParseFlags()

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
