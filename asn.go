package main

const (
	// ASN assignments for different node types.
	ASNSpine      = 4200000000
	ASNBorderLeaf = 4200000001
	ASNRouter     = 4200000002
	ASNLeafBase   = 4200001000
	ASNToRBase    = 4200010000
	ASNServerBase = 4200100000
)

// LeafASN returns the ASN for a leaf pair.
func LeafASN(pairIndex int) int {
	return ASNLeafBase + pairIndex
}

// ToRASN returns the ASN for a ToR.
func ToRASN(index int) int {
	return ASNToRBase + index
}

// ServerASN returns the ASN for a server.
func ServerASN(index int) int {
	return ASNServerBase + index
}
