package geoip

import (
	"testing"
)

func TestIsIPAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1.2.3.4", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"example.com", false},
		{"", false},
		{"256.1.1.1", false},
		{"1.1.1.1:443", true}, // handles port notation
		{"::1", false},        // no IPv6 support
	}

	for _, tt := range tests {
		result := isIPAddress(tt.input)
		if result != tt.expected {
			t.Errorf("isIPAddress(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestRegionCodeCoverage(t *testing.T) {
	// Ensure common country codes have Chinese mappings
	requiredCodes := []string{"HK", "TW", "JP", "KR", "SG", "US", "GB", "DE", "FR", "AU", "CA", "RU"}
	for _, code := range requiredCodes {
		if _, ok := RegionCode[code]; !ok {
			t.Errorf("missing region mapping for country code %q", code)
		}
	}
}
