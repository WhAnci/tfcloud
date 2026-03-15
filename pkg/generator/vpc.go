package generator

import (
	"fmt"
	"sort"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"

	"github.com/WhAnci/tfcloud/pkg/parser"
)

// VPCGenerator generates Terraform HCL for VPC resources.
type VPCGenerator struct{}

func (g *VPCGenerator) Kind() string     { return "VPC" }
func (g *VPCGenerator) Filename() string { return "main.tf" }

func (g *VPCGenerator) Generate(manifest *parser.Manifest) ([]byte, error) {
	spec, ok := manifest.Spec.(*parser.VPCSpec)
	if !ok {
		return nil, fmt.Errorf("expected VPCSpec, got %T", manifest.Spec)
	}

	f := hclwrite.NewEmptyFile()
	body := f.Body()

	g.writeTerraformBlock(body)
	body.AppendNewline()

	g.writeProvider(body, spec)
	body.AppendNewline()

	g.writeVPC(body, manifest.Metadata.Name, spec)
	body.AppendNewline()

	g.writePublicSubnets(body, manifest.Metadata.Name, spec)
	body.AppendNewline()

	g.writePrivateSubnets(body, manifest.Metadata.Name, spec)
	body.AppendNewline()

	if spec.Route.InternetGateway.Enabled {
		g.writeInternetGateway(body, manifest.Metadata.Name, spec)
		body.AppendNewline()
	}

	g.writeNATGateways(body, manifest.Metadata.Name, spec)

	g.writeRouteTables(body, manifest.Metadata.Name, spec)

	return f.Bytes(), nil
}

func (g *VPCGenerator) writeTerraformBlock(body *hclwrite.Body) {
	tfBlock := body.AppendNewBlock("terraform", nil)
	rpBlock := tfBlock.Body().AppendNewBlock("required_providers", nil)
	rpBlock.Body().SetAttributeValue("aws", cty.ObjectVal(map[string]cty.Value{
		"source": cty.StringVal("hashicorp/aws"),
	}))
}

func (g *VPCGenerator) writeProvider(body *hclwrite.Body, spec *parser.VPCSpec) {
	providerBlock := body.AppendNewBlock("provider", []string{"aws"})
	providerBlock.Body().SetAttributeValue("region", cty.StringVal(spec.Region))
}

func (g *VPCGenerator) writeVPC(body *hclwrite.Body, name string, spec *parser.VPCSpec) {
	block := body.AppendNewBlock("resource", []string{"aws_vpc", sanitize(name)})
	b := block.Body()
	b.SetAttributeValue("cidr_block", cty.StringVal(spec.CIDRBlock))
	b.SetAttributeValue("enable_dns_hostnames", cty.BoolVal(spec.DNS.Hostnames))
	b.SetAttributeValue("enable_dns_support", cty.BoolVal(spec.DNS.Resolution))
	b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": name}, spec.Tags))
}

func (g *VPCGenerator) writePublicSubnets(body *hclwrite.Body, vpcName string, spec *parser.VPCSpec) {
	vpcRef := fmt.Sprintf("aws_vpc.%s.id", sanitize(vpcName))
	for _, subnet := range spec.Public {
		block := body.AppendNewBlock("resource", []string{"aws_subnet", sanitize(subnet.Name)})
		b := block.Body()
		b.SetAttributeRaw("vpc_id", rawRef(vpcRef))
		b.SetAttributeValue("cidr_block", cty.StringVal(subnet.CIDR))
		b.SetAttributeValue("availability_zone", cty.StringVal(spec.Region+subnet.Zone))
		b.SetAttributeValue("map_public_ip_on_launch", cty.True)
		b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": subnet.Name}, spec.Tags))
		body.AppendNewline()
	}
}

func (g *VPCGenerator) writePrivateSubnets(body *hclwrite.Body, vpcName string, spec *parser.VPCSpec) {
	vpcRef := fmt.Sprintf("aws_vpc.%s.id", sanitize(vpcName))
	for _, subnet := range spec.Private {
		block := body.AppendNewBlock("resource", []string{"aws_subnet", sanitize(subnet.Name)})
		b := block.Body()
		b.SetAttributeRaw("vpc_id", rawRef(vpcRef))
		b.SetAttributeValue("cidr_block", cty.StringVal(subnet.CIDR))
		b.SetAttributeValue("availability_zone", cty.StringVal(spec.Region+subnet.Zone))
		b.SetAttributeValue("map_public_ip_on_launch", cty.False)
		b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": subnet.Name}, spec.Tags))
		body.AppendNewline()
	}
}

func (g *VPCGenerator) writeInternetGateway(body *hclwrite.Body, vpcName string, spec *parser.VPCSpec) {
	igwName := spec.Route.InternetGateway.Name
	if igwName == "" {
		igwName = vpcName + "-igw"
	}
	vpcRef := fmt.Sprintf("aws_vpc.%s.id", sanitize(vpcName))

	block := body.AppendNewBlock("resource", []string{"aws_internet_gateway", sanitize(igwName)})
	b := block.Body()
	b.SetAttributeRaw("vpc_id", rawRef(vpcRef))
	b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": igwName}, spec.Tags))
}

func (g *VPCGenerator) writeNATGateways(body *hclwrite.Body, vpcName string, spec *parser.VPCSpec) {
	strategy := spec.Route.NATGateway.Strategy
	if strategy == "" || strategy == "none" {
		return
	}

	switch strategy {
	case "regional", "single":
		g.writeRegionalNAT(body, vpcName, spec)
	case "per-az":
		g.writePerAZNAT(body, vpcName, spec)
	}
}

func (g *VPCGenerator) writeRegionalNAT(body *hclwrite.Body, vpcName string, spec *parser.VPCSpec) {
	if len(spec.Public) == 0 {
		return
	}

	natBaseName := parser.GetNATBaseName(spec.Route.NATGateway.Name)
	eipName := natBaseName + "-eip"

	// EIP
	eipBlock := body.AppendNewBlock("resource", []string{"aws_eip", sanitize(eipName)})
	eipBlock.Body().SetAttributeValue("domain", cty.StringVal("vpc"))
	eipBlock.Body().SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": eipName}, spec.Tags))
	body.AppendNewline()

	// NAT Gateway in the first public subnet
	firstPub := spec.Public[0]
	subnetRef := fmt.Sprintf("aws_subnet.%s.id", sanitize(firstPub.Name))
	eipRef := fmt.Sprintf("aws_eip.%s.id", sanitize(eipName))
	igwName := spec.Route.InternetGateway.Name
	if igwName == "" {
		igwName = vpcName + "-igw"
	}

	natBlock := body.AppendNewBlock("resource", []string{"aws_nat_gateway", sanitize(natBaseName)})
	b := natBlock.Body()
	b.SetAttributeRaw("allocation_id", rawRef(eipRef))
	b.SetAttributeRaw("subnet_id", rawRef(subnetRef))
	b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": natBaseName}, spec.Tags))
	body.AppendNewline()

	// depends_on for IGW
	b.SetAttributeRaw("depends_on", rawRefList([]string{
		fmt.Sprintf("aws_internet_gateway.%s", sanitize(igwName)),
	}))
	body.AppendNewline()
}

func (g *VPCGenerator) writePerAZNAT(body *hclwrite.Body, vpcName string, spec *parser.VPCSpec) {
	if len(spec.Public) == 0 {
		return
	}

	// Collect unique AZs from public subnets, pick first public subnet per AZ.
	azSubnet := map[string]parser.SubnetSpec{}
	azOrder := []string{}
	for _, sub := range spec.Public {
		if _, exists := azSubnet[sub.Zone]; !exists {
			azSubnet[sub.Zone] = sub
			azOrder = append(azOrder, sub.Zone)
		}
	}
	sort.Strings(azOrder)

	igwName := spec.Route.InternetGateway.Name
	if igwName == "" {
		igwName = vpcName + "-igw"
	}

	for _, zone := range azOrder {
		sub := azSubnet[zone]
		natName := parser.GetNATNameForAZ(spec.Route.NATGateway.Name, zone)
		eipName := natName + "-eip"

		// EIP
		eipBlock := body.AppendNewBlock("resource", []string{"aws_eip", sanitize(eipName)})
		eipBlock.Body().SetAttributeValue("domain", cty.StringVal("vpc"))
		eipBlock.Body().SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": eipName}, spec.Tags))
		body.AppendNewline()

		// NAT Gateway
		subnetRef := fmt.Sprintf("aws_subnet.%s.id", sanitize(sub.Name))
		eipRef := fmt.Sprintf("aws_eip.%s.id", sanitize(eipName))

		natBlock := body.AppendNewBlock("resource", []string{"aws_nat_gateway", sanitize(natName)})
		b := natBlock.Body()
		b.SetAttributeRaw("allocation_id", rawRef(eipRef))
		b.SetAttributeRaw("subnet_id", rawRef(subnetRef))
		b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": natName}, spec.Tags))
		b.SetAttributeRaw("depends_on", rawRefList([]string{
			fmt.Sprintf("aws_internet_gateway.%s", sanitize(igwName)),
		}))
		body.AppendNewline()
	}
}

func (g *VPCGenerator) writeRouteTables(body *hclwrite.Body, vpcName string, spec *parser.VPCSpec) {
	vpcRef := fmt.Sprintf("aws_vpc.%s.id", sanitize(vpcName))

	// Public route tables
	if spec.Route.PublicRouteTablePerAZ {
		g.writePerAZPublicRouteTables(body, vpcName, vpcRef, spec)
	} else {
		g.writeSharedPublicRouteTable(body, vpcName, vpcRef, spec)
	}

	// Private route tables
	if spec.Route.PrivateRouteTablePerAZ {
		g.writePerAZPrivateRouteTables(body, vpcName, vpcRef, spec)
	} else {
		g.writeSharedPrivateRouteTable(body, vpcName, vpcRef, spec)
	}
}

func (g *VPCGenerator) writeSharedPublicRouteTable(body *hclwrite.Body, vpcName, vpcRef string, spec *parser.VPCSpec) {
	rtName := vpcName + "-public-rt"

	block := body.AppendNewBlock("resource", []string{"aws_route_table", sanitize(rtName)})
	b := block.Body()
	b.SetAttributeRaw("vpc_id", rawRef(vpcRef))
	b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": rtName}, spec.Tags))
	body.AppendNewline()

	// Route to IGW
	if spec.Route.InternetGateway.Enabled {
		igwName := spec.Route.InternetGateway.Name
		if igwName == "" {
			igwName = vpcName + "-igw"
		}
		routeName := rtName + "-igw-route"
		routeBlock := body.AppendNewBlock("resource", []string{"aws_route", sanitize(routeName)})
		rb := routeBlock.Body()
		rb.SetAttributeRaw("route_table_id", rawRef(fmt.Sprintf("aws_route_table.%s.id", sanitize(rtName))))
		rb.SetAttributeValue("destination_cidr_block", cty.StringVal("0.0.0.0/0"))
		rb.SetAttributeRaw("gateway_id", rawRef(fmt.Sprintf("aws_internet_gateway.%s.id", sanitize(igwName))))
		body.AppendNewline()
	}

	// Associations
	for _, subnet := range spec.Public {
		assocName := sanitize(subnet.Name) + "_assoc"
		assocBlock := body.AppendNewBlock("resource", []string{"aws_route_table_association", assocName})
		ab := assocBlock.Body()
		ab.SetAttributeRaw("subnet_id", rawRef(fmt.Sprintf("aws_subnet.%s.id", sanitize(subnet.Name))))
		ab.SetAttributeRaw("route_table_id", rawRef(fmt.Sprintf("aws_route_table.%s.id", sanitize(rtName))))
		body.AppendNewline()
	}
}

func (g *VPCGenerator) writePerAZPublicRouteTables(body *hclwrite.Body, vpcName, vpcRef string, spec *parser.VPCSpec) {
	igwName := spec.Route.InternetGateway.Name
	if igwName == "" {
		igwName = vpcName + "-igw"
	}

	for _, subnet := range spec.Public {
		rtName := vpcName + "-public-rt-" + subnet.Zone

		block := body.AppendNewBlock("resource", []string{"aws_route_table", sanitize(rtName)})
		b := block.Body()
		b.SetAttributeRaw("vpc_id", rawRef(vpcRef))
		b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": rtName}, spec.Tags))
		body.AppendNewline()

		if spec.Route.InternetGateway.Enabled {
			routeName := rtName + "-igw-route"
			routeBlock := body.AppendNewBlock("resource", []string{"aws_route", sanitize(routeName)})
			rb := routeBlock.Body()
			rb.SetAttributeRaw("route_table_id", rawRef(fmt.Sprintf("aws_route_table.%s.id", sanitize(rtName))))
			rb.SetAttributeValue("destination_cidr_block", cty.StringVal("0.0.0.0/0"))
			rb.SetAttributeRaw("gateway_id", rawRef(fmt.Sprintf("aws_internet_gateway.%s.id", sanitize(igwName))))
			body.AppendNewline()
		}

		assocName := sanitize(subnet.Name) + "_assoc"
		assocBlock := body.AppendNewBlock("resource", []string{"aws_route_table_association", assocName})
		ab := assocBlock.Body()
		ab.SetAttributeRaw("subnet_id", rawRef(fmt.Sprintf("aws_subnet.%s.id", sanitize(subnet.Name))))
		ab.SetAttributeRaw("route_table_id", rawRef(fmt.Sprintf("aws_route_table.%s.id", sanitize(rtName))))
		body.AppendNewline()
	}
}

func (g *VPCGenerator) writeSharedPrivateRouteTable(body *hclwrite.Body, vpcName, vpcRef string, spec *parser.VPCSpec) {
	rtName := vpcName + "-private-rt"

	block := body.AppendNewBlock("resource", []string{"aws_route_table", sanitize(rtName)})
	b := block.Body()
	b.SetAttributeRaw("vpc_id", rawRef(vpcRef))
	b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": rtName}, spec.Tags))
	body.AppendNewline()

	// Route to NAT (if not "none")
	strategy := spec.Route.NATGateway.Strategy
	if strategy != "" && strategy != "none" {
		natBaseName := parser.GetNATBaseName(spec.Route.NATGateway.Name)
		routeName := rtName + "-nat-route"
		routeBlock := body.AppendNewBlock("resource", []string{"aws_route", sanitize(routeName)})
		rb := routeBlock.Body()
		rb.SetAttributeRaw("route_table_id", rawRef(fmt.Sprintf("aws_route_table.%s.id", sanitize(rtName))))
		rb.SetAttributeValue("destination_cidr_block", cty.StringVal("0.0.0.0/0"))
		// For shared private RT, always point to the regional/single NAT
		if strategy == "per-az" && len(spec.Public) > 0 {
			// Use first AZ's NAT
			firstZone := spec.Public[0].Zone
			natName := parser.GetNATNameForAZ(spec.Route.NATGateway.Name, firstZone)
			rb.SetAttributeRaw("nat_gateway_id", rawRef(fmt.Sprintf("aws_nat_gateway.%s.id", sanitize(natName))))
		} else {
			rb.SetAttributeRaw("nat_gateway_id", rawRef(fmt.Sprintf("aws_nat_gateway.%s.id", sanitize(natBaseName))))
		}
		body.AppendNewline()
	}

	// Associations
	for _, subnet := range spec.Private {
		assocName := sanitize(subnet.Name) + "_assoc"
		assocBlock := body.AppendNewBlock("resource", []string{"aws_route_table_association", assocName})
		ab := assocBlock.Body()
		ab.SetAttributeRaw("subnet_id", rawRef(fmt.Sprintf("aws_subnet.%s.id", sanitize(subnet.Name))))
		ab.SetAttributeRaw("route_table_id", rawRef(fmt.Sprintf("aws_route_table.%s.id", sanitize(rtName))))
		body.AppendNewline()
	}
}

func (g *VPCGenerator) writePerAZPrivateRouteTables(body *hclwrite.Body, vpcName, vpcRef string, spec *parser.VPCSpec) {
	strategy := spec.Route.NATGateway.Strategy
	natBaseName := parser.GetNATBaseName(spec.Route.NATGateway.Name)

	for _, subnet := range spec.Private {
		rtName := vpcName + "-private-rt-" + subnet.Zone

		block := body.AppendNewBlock("resource", []string{"aws_route_table", sanitize(rtName)})
		b := block.Body()
		b.SetAttributeRaw("vpc_id", rawRef(vpcRef))
		b.SetAttributeValue("tags", g.mergeTags(map[string]string{"Name": rtName}, spec.Tags))
		body.AppendNewline()

		// Route to NAT
		if strategy != "" && strategy != "none" {
			routeName := rtName + "-nat-route"
			routeBlock := body.AppendNewBlock("resource", []string{"aws_route", sanitize(routeName)})
			rb := routeBlock.Body()
			rb.SetAttributeRaw("route_table_id", rawRef(fmt.Sprintf("aws_route_table.%s.id", sanitize(rtName))))
			rb.SetAttributeValue("destination_cidr_block", cty.StringVal("0.0.0.0/0"))

			if strategy == "per-az" {
				natName := parser.GetNATNameForAZ(spec.Route.NATGateway.Name, subnet.Zone)
				rb.SetAttributeRaw("nat_gateway_id", rawRef(fmt.Sprintf("aws_nat_gateway.%s.id", sanitize(natName))))
			} else {
				rb.SetAttributeRaw("nat_gateway_id", rawRef(fmt.Sprintf("aws_nat_gateway.%s.id", sanitize(natBaseName))))
			}
			body.AppendNewline()
		}

		// Association
		assocName := sanitize(subnet.Name) + "_assoc"
		assocBlock := body.AppendNewBlock("resource", []string{"aws_route_table_association", assocName})
		ab := assocBlock.Body()
		ab.SetAttributeRaw("subnet_id", rawRef(fmt.Sprintf("aws_subnet.%s.id", sanitize(subnet.Name))))
		ab.SetAttributeRaw("route_table_id", rawRef(fmt.Sprintf("aws_route_table.%s.id", sanitize(rtName))))
		body.AppendNewline()
	}
}

// mergeTags creates a cty.Value from merged resource and common tags.
func (g *VPCGenerator) mergeTags(resourceTags, commonTags map[string]string) cty.Value {
	merged := map[string]cty.Value{}
	for k, v := range commonTags {
		merged[k] = cty.StringVal(v)
	}
	for k, v := range resourceTags {
		merged[k] = cty.StringVal(v)
	}
	if len(merged) == 0 {
		return cty.EmptyObjectVal
	}
	return cty.ObjectVal(merged)
}

// sanitize converts a name to a valid Terraform resource identifier.
// Replaces hyphens and dots with underscores.
func sanitize(name string) string {
	result := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result[i] = c
		} else {
			result[i] = '_'
		}
	}
	return string(result)
}

// rawRef creates an hclwrite token sequence for a Terraform reference (no quotes).
func rawRef(ref string) hclwrite.Tokens {
	return hclwrite.TokensForTraversal(parseTraversal(ref))
}

// rawRefList creates tokens for a list of references like [ref1, ref2].
func rawRefList(refs []string) hclwrite.Tokens {
	tokens := hclwrite.Tokens{
		{Type: tokenForOpenBracket(), Bytes: []byte("[")},
	}
	for i, ref := range refs {
		if i > 0 {
			tokens = append(tokens, &hclwrite.Token{Type: tokenForComma(), Bytes: []byte(",")})
			tokens = append(tokens, &hclwrite.Token{Type: tokenForNewline(), Bytes: []byte(" ")})
		}
		tokens = append(tokens, hclwrite.TokensForTraversal(parseTraversal(ref))...)
	}
	tokens = append(tokens, &hclwrite.Token{Type: tokenForCloseBracket(), Bytes: []byte("]")})
	return tokens
}
