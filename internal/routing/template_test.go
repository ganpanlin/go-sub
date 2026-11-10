package routing

import (
	"strings"
	"testing"
)

func TestGetTemplates(t *testing.T) {
	templates := GetTemplates()
	if len(templates) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(templates))
	}

	ids := map[string]bool{}
	for _, tpl := range templates {
		if tpl.ID == "" || tpl.Name == "" || tpl.Description == "" {
			t.Fatalf("template missing identity fields: %+v", tpl)
		}
		if ids[tpl.ID] {
			t.Fatalf("duplicate template id: %s", tpl.ID)
		}
		ids[tpl.ID] = true
		if len(tpl.Rules) == 0 {
			t.Fatalf("template %s has no rules", tpl.ID)
		}
		for _, r := range tpl.Rules {
			if r.Name == "" {
				t.Fatalf("template %s has rule without name", tpl.ID)
			}
			if len(r.Urls) == 0 && len(r.Payload) == 0 {
				t.Fatalf("template %s rule %q has neither urls nor payload", tpl.ID, r.Name)
			}
			for _, u := range r.Urls {
				if !strings.HasPrefix(u, "https://") {
					t.Fatalf("template %s rule %q has invalid url %q", tpl.ID, r.Name, u)
				}
			}
		}
	}

	// Expected ids
	for _, want := range []string{"minimal", "balanced", "full"} {
		if !ids[want] {
			t.Fatalf("missing template id %q", want)
		}
	}
}

func TestTemplateSizes(t *testing.T) {
	templates := map[string]Template{}
	for _, tpl := range GetTemplates() {
		templates[tpl.ID] = tpl
	}
	if len(templates["minimal"].Rules) != 1 {
		t.Fatalf("minimal should have exactly 1 rule, got %d", len(templates["minimal"].Rules))
	}
	if len(templates["balanced"].Rules) < 5 {
		t.Fatalf("balanced should have at least 5 rules, got %d", len(templates["balanced"].Rules))
	}
	if len(templates["full"].Rules) < 10 {
		t.Fatalf("full should cover all categories (>=10), got %d", len(templates["full"].Rules))
	}
}

func TestGetTemplateUnknown(t *testing.T) {
	if GetTemplate("nope") != nil {
		t.Fatal("unknown template should return nil")
	}
	if tpl := GetTemplate("balanced"); tpl == nil || tpl.ID != "balanced" {
		t.Fatal("balanced template should be found")
	}
}

func TestApplyTemplate(t *testing.T) {
	m := GetManager()
	before := len(m.List())

	p, err := m.ApplyTemplate("balanced", "我的均衡分流")
	if err != nil {
		t.Fatalf("ApplyTemplate: %v", err)
	}
	if p.Name != "我的均衡分流" {
		t.Fatalf("expected custom name, got %q", p.Name)
	}
	if p.ID == "" {
		t.Fatal("profile should get an ID")
	}
	if len(m.List()) != before+1 {
		t.Fatalf("profile count should grow by 1, got %d -> %d", before, len(m.List()))
	}

	// Persisted profile must be retrievable and match
	stored := m.Get(p.ID)
	if stored == nil || len(stored.Rules) == 0 {
		t.Fatal("applied profile not found in manager")
	}

	// Deep copy: mutating the stored profile must not mutate the template
	stored.Rules[0].Name = "MUTATED"
	fresh := GetTemplate("balanced")
	if fresh.Rules[0].Name == "MUTATED" {
		t.Fatal("template rules were mutated by profile edit — deep copy missing")
	}

	// Default name falls back to template name
	p2, err := m.ApplyTemplate("minimal", "")
	if err != nil {
		t.Fatalf("ApplyTemplate minimal: %v", err)
	}
	if p2.Name != "极简模式" {
		t.Fatalf("expected template default name, got %q", p2.Name)
	}
}

func TestApplyTemplateUnknown(t *testing.T) {
	if _, err := GetManager().ApplyTemplate("nope", "x"); err == nil {
		t.Fatal("unknown template should error")
	}
}

func TestTemplateBuildConfigCompatible(t *testing.T) {
	// Every template's rules must survive BuildConfig without panics
	// and produce at least the fixed base groups + MATCH rule.
	for _, tpl := range GetTemplates() {
		rp := &RoutingProfile{ID: "tpl-test", Name: tpl.Name, Rules: tpl.Rules}
		groups, providers, rules := BuildConfig(rp, []string{"node-1"})
		if len(groups) == 0 || len(rules) == 0 {
			t.Fatalf("template %s produced empty config", tpl.ID)
		}
		if providers == nil {
			t.Fatalf("template %s: providers map should be initialized", tpl.ID)
		}
		hasMatch := false
		for _, r := range rules {
			if len(r) >= 5 && r[:5] == "MATCH" {
				hasMatch = true
			}
		}
		if !hasMatch {
			t.Fatalf("template %s: missing MATCH rule", tpl.ID)
		}
	}
}
