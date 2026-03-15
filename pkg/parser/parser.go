package parser

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Manifest represents the top-level Kubernetes-style YAML manifest.
type Manifest struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       any      `yaml:"spec"` // Parsed based on Kind
}

// Metadata contains the resource metadata.
type Metadata struct {
	Name string `yaml:"name"`
}

// RawManifest is used for initial parsing to determine the Kind before
// deserializing the Spec into the correct struct.
type RawManifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   Metadata     `yaml:"metadata"`
	Spec       yaml.Node    `yaml:"spec"`
}

// VPCSpec defines the full VPC specification.
type VPCSpec struct {
	Region        string         `yaml:"region"`
	CIDRBlock     string         `yaml:"cidrBlock"`
	IPv6CIDRBlock bool           `yaml:"ipv6CidrBlock"`
	Public        []SubnetSpec   `yaml:"public"`
	Private       []SubnetSpec   `yaml:"private"`
	Route         RouteSpec      `yaml:"route"`
	DNS           DNSSpec        `yaml:"dns"`
	Tags          map[string]string `yaml:"tags"`
}

// SubnetSpec defines a subnet within the VPC.
type SubnetSpec struct {
	Name string `yaml:"name"`
	CIDR string `yaml:"cidr"`
	Zone string `yaml:"zone"`
}

// RouteSpec defines routing configuration.
type RouteSpec struct {
	InternetGateway       IGWSpec  `yaml:"internetGateway"`
	NATGateway            NATSpec  `yaml:"natGateway"`
	PublicRouteTablePerAZ  bool    `yaml:"publicRouteTablePerAz"`
	PrivateRouteTablePerAZ bool    `yaml:"privateRouteTablePerAz"`
}

// IGWSpec defines the Internet Gateway configuration.
type IGWSpec struct {
	Enabled bool   `yaml:"enabled"`
	Name    string `yaml:"name"`
}

// NATSpec defines the NAT Gateway configuration.
type NATSpec struct {
	Strategy string  `yaml:"strategy"` // regional, single, per-az, none
	Name     any     `yaml:"name"`     // string or list of maps for per-az
}

// DNSSpec defines DNS settings for the VPC.
type DNSSpec struct {
	Hostnames  bool `yaml:"hostnames"`
	Resolution bool `yaml:"resolution"`
}

// ParseFile reads and parses a YAML manifest file.
func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return Parse(data)
}

// Parse parses YAML bytes into a Manifest.
func Parse(data []byte) (*Manifest, error) {
	var raw RawManifest
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	manifest := &Manifest{
		APIVersion: raw.APIVersion,
		Kind:       raw.Kind,
		Metadata:   raw.Metadata,
	}

	switch raw.Kind {
	case "VPC":
		var spec VPCSpec
		if err := raw.Spec.Decode(&spec); err != nil {
			return nil, fmt.Errorf("failed to parse VPC spec: %w", err)
		}
		manifest.Spec = &spec
	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", raw.Kind)
	}

	return manifest, nil
}

// GetNATNameForAZ returns the NAT gateway name for a specific AZ.
// If the name is a string, it appends the zone suffix.
// If the name is a list of maps, it looks up the zone's name.
func GetNATNameForAZ(natName any, zone string) string {
	switch v := natName.(type) {
	case string:
		return fmt.Sprintf("%s-%s", v, zone)
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if name, ok := m[zone]; ok {
					return fmt.Sprintf("%v", name)
				}
			}
		}
		// Fallback to using zone suffix with empty prefix
		return fmt.Sprintf("nat-%s", zone)
	default:
		return fmt.Sprintf("nat-%s", zone)
	}
}

// GetNATBaseName returns the base NAT name as a string.
func GetNATBaseName(natName any) string {
	switch v := natName.(type) {
	case string:
		return v
	default:
		return "nat"
	}
}
