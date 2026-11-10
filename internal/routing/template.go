package routing

import "fmt"

// Template is a built-in routing configuration preset. Applying a template
// creates a ready-to-use RoutingProfile in one click, sourced from the
// built-in rule catalog (defaultCatalog).
type Template struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Rules       []RuleItem `json:"rules"`
}

// templateGroupSpec picks presets from the catalog for a balanced-template group.
type templateGroupSpec struct {
	Group   string   // proxy-group name
	CatID   string   // catalog category id
	Presets []string // catalog preset ids
	Extra   []string // extra proxies for the group (e.g. REJECT)
}

// balancedSpec defines which catalog presets make up the balanced template.
var balancedSpec = []templateGroupSpec{
	{Group: "广告拦截", CatID: "privacy", Presets: []string{"advertising_lite"}, Extra: []string{"REJECT"}},
	{Group: "AI服务", CatID: "ai", Presets: []string{"openai", "claude", "gemini"}},
	{Group: "流媒体", CatID: "streaming", Presets: []string{"youtube", "netflix", "disney"}},
	{Group: "社交平台", CatID: "social", Presets: []string{"telegram", "twitter"}},
	{Group: "谷歌服务", CatID: "google", Presets: []string{"google"}},
	{Group: "开发工具", CatID: "dev", Presets: []string{"github"}},
	{Group: "微软服务", CatID: "microsoft", Presets: []string{"microsoft", "onedrive"}},
	{Group: "苹果服务", CatID: "apple", Presets: []string{"apple"}},
}

// GetTemplates returns the built-in routing templates.
// Templates derive from the built-in default catalog so their rule URLs stay valid
// even if the user has customized data/catalog.json.
func GetTemplates() []Template {
	catalog := defaultCatalog()

	// Minimal: ad-block only; base groups + MATCH catch-all handle the rest.
	minimal := Template{
		ID:          "minimal",
		Name:        "极简模式",
		Description: "仅保留广告拦截，其余流量走兜底分组。节点少、追求客户端性能时使用。",
		Rules:       []RuleItem{buildSpecRule(catalog, templateGroupSpec{Group: "广告拦截", CatID: "privacy", Presets: []string{"advertising_lite"}, Extra: []string{"REJECT"}})},
	}

	// Balanced: curated everyday categories.
	var balancedRules []RuleItem
	for _, spec := range balancedSpec {
		balancedRules = append(balancedRules, buildSpecRule(catalog, spec))
	}
	balanced := Template{
		ID:          "balanced",
		Name:        "均衡模式",
		Description: "广告拦截 + AI/流媒体/社交/谷歌走代理，微软/苹果/开发工具直连。适合日常使用。",
		Rules:       balancedRules,
	}

	// Full: one proxy-group per catalog category, covering every built-in preset.
	var fullRules []RuleItem
	for _, cat := range catalog {
		item := RuleItem{Name: cat.Name, GFW: cat.GFW}
		for _, preset := range cat.Rules {
			item.Urls = append(item.Urls, presetURLs(preset)...)
			item.Payload = append(item.Payload, preset.Payload...)
		}
		if cat.ID == "privacy" {
			item.ExtraProxies = []string{"REJECT"}
		}
		fullRules = append(fullRules, item)
	}
	full := Template{
		ID:          "full",
		Name:        "完整模式",
		Description: "按内置规则库全部分类生成策略组，覆盖全部 58 条规则。分组最多、分流最细。",
		Rules:       fullRules,
	}

	return []Template{minimal, balanced, full}
}

// GetTemplate returns a template by id, or nil if unknown.
func GetTemplate(id string) *Template {
	for _, t := range GetTemplates() {
		if t.ID == id {
			t := t
			return &t
		}
	}
	return nil
}

// ApplyTemplate creates a new RoutingProfile from a built-in template.
// Rules are deep-copied so later edits never mutate the template.
func (m *Manager) ApplyTemplate(templateID, name string) (*RoutingProfile, error) {
	t := GetTemplate(templateID)
	if t == nil {
		return nil, fmt.Errorf("unknown template: %s", templateID)
	}
	p := &RoutingProfile{
		Name:  name,
		Rules: make([]RuleItem, len(t.Rules)),
	}
	if p.Name == "" {
		p.Name = t.Name
	}
	copy(p.Rules, t.Rules)
	if err := m.Add(p); err != nil {
		return nil, err
	}
	return p, nil
}

// buildSpecRule converts a templateGroupSpec into a RuleItem using catalog lookups.
// Presets that are missing from the catalog (renamed/removed upstream) are skipped.
func buildSpecRule(catalog []RuleCategory, spec templateGroupSpec) RuleItem {
	item := RuleItem{Name: spec.Group, GFW: presetGFW(catalog, spec.CatID), ExtraProxies: spec.Extra}
	for _, presetID := range spec.Presets {
		preset, ok := findPreset(catalog, spec.CatID, presetID)
		if !ok {
			continue
		}
		item.Urls = append(item.Urls, presetURLs(preset)...)
		item.Payload = append(item.Payload, preset.Payload...)
	}
	return item
}

func findPreset(catalog []RuleCategory, catID, presetID string) (RulePreset, bool) {
	for _, cat := range catalog {
		if cat.ID != catID {
			continue
		}
		for _, preset := range cat.Rules {
			if preset.ID == presetID {
				return preset, true
			}
		}
	}
	return RulePreset{}, false
}

func presetGFW(catalog []RuleCategory, catID string) bool {
	for _, cat := range catalog {
		if cat.ID == catID {
			return cat.GFW
		}
	}
	return false
}

func presetURLs(preset RulePreset) []string {
	if len(preset.URLs) > 0 {
		return preset.URLs
	}
	if preset.URL != "" {
		return []string{preset.URL}
	}
	return nil
}
