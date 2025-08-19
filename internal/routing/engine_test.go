package routing

import (
	"encoding/json"
	"testing"
)

func TestCreatePayloadRulesMatchesProxyGrepEngine(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		group    string
		expected string
	}{
		{
			name:     "ip cidr",
			payload:  "IP-CIDR,10.0.0.0/8",
			group:    "cn",
			expected: "IP-CIDR,10.0.0.0/8,cn",
		},
		{
			name:     "ip cidr no resolve",
			payload:  "IP-CIDR,10.0.0.0/8,no-resolve",
			group:    "cn",
			expected: "IP-CIDR,10.0.0.0/8,cn,no-resolve",
		},
		{
			name:     "group comma is sanitized",
			payload:  "DOMAIN-SUFFIX,example.com",
			group:    "foo,bar",
			expected: "DOMAIN-SUFFIX,example.com,foo-bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := createPayloadRules(tt.payload, tt.group)
			if len(got) != 1 {
				t.Fatalf("expected one rule, got %#v", got)
			}
			if got[0] != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got[0])
			}
		})
	}
}

func TestBuildConfigPayloadOverridesUrls(t *testing.T) {
	rp := &RoutingProfile{
		Name: "test",
		Rules: []RuleItem{
			{
				Name:    "cn",
				Urls:    StringList{"https://example.com/rule.yaml"},
				Payload: StringList{"DOMAIN-SUFFIX,openai.com"},
			},
		},
	}

	_, providers, rules := BuildConfig(rp, nil)
	if len(providers) != 0 {
		t.Fatalf("payload should override urls, got providers %#v", providers)
	}
	assertContainsRule(t, rules, "DOMAIN-SUFFIX,openai.com,cn")
}

func TestStringListAcceptsStringOrArray(t *testing.T) {
	var item RuleItem
	err := json.Unmarshal([]byte(`{
		"name": "广告拦截",
		"urls": "https://example.com/rule.yaml",
		"payload": ["DOMAIN-SUFFIX,example.com"],
		"extraProxies": "REJECT"
	}`), &item)
	if err != nil {
		t.Fatal(err)
	}

	if len(item.Urls) != 1 || item.Urls[0] != "https://example.com/rule.yaml" {
		t.Fatalf("unexpected urls: %#v", item.Urls)
	}
	if len(item.Payload) != 1 || item.Payload[0] != "DOMAIN-SUFFIX,example.com" {
		t.Fatalf("unexpected payload: %#v", item.Payload)
	}
	if len(item.ExtraProxies) != 1 || item.ExtraProxies[0] != "REJECT" {
		t.Fatalf("unexpected extra proxies: %#v", item.ExtraProxies)
	}
}

func assertContainsRule(t *testing.T, rules []string, want string) {
	t.Helper()
	for _, rule := range rules {
		if rule == want {
			return
		}
	}
	t.Fatalf("expected rules to contain %q, got %#v", want, rules)
}
