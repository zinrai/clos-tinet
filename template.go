package main

import (
	"bytes"
	"os"
	"text/template"

	"github.com/goccy/go-yaml"
)

// Templates holds BIRD configuration templates for each role.
type Templates struct {
	Spine  string `yaml:"spine"`
	Leaf   string `yaml:"leaf"`
	BL     string `yaml:"bl"`
	ToR    string `yaml:"tor"`
	Server string `yaml:"server"`
	Router string `yaml:"router"`
}

// TemplateData holds data for template rendering.
type TemplateData struct {
	RouterID  string
	ASN       int
	Neighbors []Neighbor
}

// LoadTemplates loads templates from a YAML file.
func LoadTemplates(path string) (*Templates, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var t Templates
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, err
	}

	return &t, nil
}

// Render renders a template with the given data.
func (t *Templates) Render(role string, data TemplateData) (string, error) {
	var tmplStr string
	switch role {
	case "spine":
		tmplStr = t.Spine
	case "leaf":
		tmplStr = t.Leaf
	case "bl":
		tmplStr = t.BL
	case "tor":
		tmplStr = t.ToR
	case "server":
		tmplStr = t.Server
	case "router":
		tmplStr = t.Router
	default:
		tmplStr = ""
	}

	tmpl, err := template.New(role).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
