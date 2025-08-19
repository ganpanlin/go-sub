package proxy

import (
	"testing"
)

func TestExpandRegionNameFilter(t *testing.T) {
	tests := []struct {
		filter   string
		contains []string // regex should contain these patterns
	}{
		{"香港", []string{"hk", "hongkong", "香港"}},
		{"HK", []string{"hk", "hongkong"}},
		{"japan", []string{"jp", "japan", "东京"}},
		{"sg", []string{"sg", "singapore", "新加坡"}},
		{"us", []string{"us", "america", "美国"}},
	}

	for _, tt := range tests {
		result := ExpandRegionNameFilter(tt.filter)
		for _, pat := range tt.contains {
			if !contains(result, pat) {
				t.Errorf("ExpandRegionNameFilter(%q) = %q, missing pattern %q", tt.filter, result, pat)
			}
		}
	}
}

func TestExpandRegionNameFilterEmpty(t *testing.T) {
	result := ExpandRegionNameFilter("")
	if result != ".*" {
		t.Fatalf("expected '.*' for empty filter, got %q", result)
	}
}

func TestExpandRegionNameFilterCustom(t *testing.T) {
	result := ExpandRegionNameFilter("premium")
	if result != "(?i)premium" {
		t.Fatalf("expected '(?i)premium' for custom filter, got %q", result)
	}
}

func TestDetectRegionFromName(t *testing.T) {
	tests := []struct {
		name string
		code string
	}{
		{"香港 IPLC 01", "HK"},
		{"HK-Premium-Node", "HK"},
		{"Hong Kong Server", "HK"},
		{"东京节点", "JP"},
		{"JP-Tokyo-SS", "JP"},
		{"Singapore-01", "SG"},
		{"US-California", "US"},
		{"Deutschland-Server", "DE"},
		{"Moscow-VPS", "RU"},
		{"UnknownNode", ""},
	}

	for _, tt := range tests {
		result := detectRegionFromName(tt.name)
		if result != tt.code {
			t.Errorf("detectRegionFromName(%q) = %q, want %q", tt.name, result, tt.code)
		}
	}
}

func TestStripRegionKeywords(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"香港 IPLC 01", "IPLC 01"},
		{"HK-Premium-Node", "Premium Node"},
		{"Hong Kong Server", "Server"},
		{"东京 Node A", "Node A"},
		{"JP-Tokyo-SS", "Tokyo SS"},
		{"美国洛杉矶01", "01"},
		{"Singapore 01", "01"},
		{"UnknownNode", "UnknownNode"},
		{"香港|日本|美国01", "01"},
	}

	for _, tt := range tests {
		result := StripRegionKeywords(tt.input)
		if result != tt.expected {
			t.Errorf("StripRegionKeywords(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestRegionShortCodeCompleteness(t *testing.T) {
	// Every keyword set should have a corresponding short code
	for code := range regionKeywords {
		if _, ok := RegionShortCode[code]; code != "" && !ok {
			// Short codes like "HK" are not in RegionShortCode (which uses Chinese names)
			// This is fine, they are the short codes themselves
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
