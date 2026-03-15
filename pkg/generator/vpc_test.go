package generator

import (
	"os"
	"strings"
	"testing"

	"github.com/WhAnci/tfcloud/pkg/parser"
)

func TestVPCGenerate(t *testing.T) {
	yamlData, err := os.ReadFile("../../examples/vpc.yaml")
	if err != nil {
		t.Fatalf("failed to read example YAML: %v", err)
	}

	manifest, err := parser.Parse(yamlData)
	if err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	if manifest.Kind != "VPC" {
		t.Fatalf("expected kind VPC, got %s", manifest.Kind)
	}
	if manifest.Metadata.Name != "wh-vpc-01" {
		t.Fatalf("expected name wh-vpc-01, got %s", manifest.Metadata.Name)
	}

	gen, err := GetGenerator("VPC")
	if err != nil {
		t.Fatalf("failed to get VPC generator: %v", err)
	}

	hclBytes, err := gen.Generate(manifest)
	if err != nil {
		t.Fatalf("failed to generate terraform: %v", err)
	}

	if len(hclBytes) == 0 {
		t.Fatal("generated terraform is empty")
	}

	hcl := string(hclBytes)

	// Verify key resources exist
	checks := []string{
		`resource "aws_vpc" "wh_vpc_01"`,
		`resource "aws_subnet" "wsi_public_subnet_a"`,
		`resource "aws_subnet" "wsi_public_subnet_c"`,
		`resource "aws_subnet" "wsi_private_subnet_a"`,
		`resource "aws_subnet" "wsi_private_subnet_c"`,
		`resource "aws_internet_gateway" "wsi_igw"`,
		`resource "aws_eip" "wsi_ngw_eip"`,
		`resource "aws_nat_gateway" "wsi_ngw"`,
		`resource "aws_route_table"`,
		`resource "aws_route_table_association"`,
		`provider "aws"`,
		`region = "ap-northeast-2"`,
		`"10.0.0.0/16"`,
		`enable_dns_hostnames = true`,
		`enable_dns_support`,
		`map_public_ip_on_launch = true`,
		`required_providers`,
		`source = "hashicorp/aws"`,
	}

	for _, check := range checks {
		if !strings.Contains(hcl, check) {
			t.Errorf("generated HCL missing expected content: %s", check)
		}
	}

	// Verify per-AZ private route tables (privateRouteTablePerAz: true)
	if !strings.Contains(hcl, `resource "aws_route_table" "wh_vpc_01_private_rt_a"`) {
		t.Error("expected per-AZ private route table for zone a")
	}
	if !strings.Contains(hcl, `resource "aws_route_table" "wh_vpc_01_private_rt_c"`) {
		t.Error("expected per-AZ private route table for zone c")
	}

	// Verify shared public route table (publicRouteTablePerAz: false)
	if !strings.Contains(hcl, `resource "aws_route_table" "wh_vpc_01_public_rt"`) {
		t.Error("expected shared public route table")
	}

	t.Logf("Generated %d bytes of HCL", len(hclBytes))
}

func TestSanitize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-vpc-01", "my_vpc_01"},
		{"subnet.name", "subnet_name"},
		{"valid_name", "valid_name"},
		{"name123", "name123"},
	}

	for _, tc := range tests {
		result := sanitize(tc.input)
		if result != tc.expected {
			t.Errorf("sanitize(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestGetNATNameForAZ(t *testing.T) {
	// String name
	name := parser.GetNATNameForAZ("wsi-ngw", "a")
	if name != "wsi-ngw-a" {
		t.Errorf("expected wsi-ngw-a, got %s", name)
	}

	// List name
	listName := []any{
		map[string]any{"a": "custom-nat-a"},
		map[string]any{"c": "custom-nat-c"},
	}
	name = parser.GetNATNameForAZ(listName, "a")
	if name != "custom-nat-a" {
		t.Errorf("expected custom-nat-a, got %s", name)
	}
	name = parser.GetNATNameForAZ(listName, "c")
	if name != "custom-nat-c" {
		t.Errorf("expected custom-nat-c, got %s", name)
	}
}
