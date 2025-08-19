package routing

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"go-sub/internal/datastore"
	"sort"
	"sync"
)

// Manager stores and manages routing profiles.
type Manager struct {
	profiles map[string]*RoutingProfile
	mu       sync.RWMutex
}

var globalManager = &Manager{
	profiles: make(map[string]*RoutingProfile),
}

func GetManager() *Manager { return globalManager }

const DefaultRoutingID = "default"

// RuleItem defines a single routing rule entry.
type RuleItem struct {
	Name         string     `json:"name"`
	GFW          bool       `json:"gfw,omitempty"`
	Urls         StringList `json:"urls,omitempty"`
	Payload      StringList `json:"payload,omitempty"`
	ExtraProxies StringList `json:"extraProxies,omitempty"`
}

// RoutingProfile is a named collection of routing rules.
type RoutingProfile struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Rules []RuleItem `json:"rules"`
}

// StringList accepts either "value" or ["value"] in JSON.
type StringList []string

func (s *StringList) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}

	var one string
	if err := json.Unmarshal(data, &one); err == nil {
		if one == "" {
			*s = nil
		} else {
			*s = []string{one}
		}
		return nil
	}

	var many []string
	if err := json.Unmarshal(data, &many); err != nil {
		return err
	}
	*s = many
	return nil
}

func LoadFromConfig() {
	var profiles []*RoutingProfile
	if err := datastore.ReadJSON("routing.json", &profiles); err != nil {
		return
	}
	m := GetManager()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range profiles {
		m.profiles[p.ID] = p
	}
}

func saveToDisk() error {
	m := GetManager()
	m.mu.RLock()
	list := make([]*RoutingProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		list = append(list, p)
	}
	m.mu.RUnlock()
	sortRoutingProfiles(list)

	return datastore.Save("routing.json", list)
}

// --- CRUD ---

func (m *Manager) List() []*RoutingProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*RoutingProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		out = append(out, p)
	}
	sortRoutingProfiles(out)
	return out
}

func sortRoutingProfiles(profiles []*RoutingProfile) {
	sort.SliceStable(profiles, func(i, j int) bool {
		if profiles[i].ID == DefaultRoutingID {
			return true
		}
		if profiles[j].ID == DefaultRoutingID {
			return false
		}
		if profiles[i].Name == profiles[j].Name {
			return profiles[i].ID < profiles[j].ID
		}
		return profiles[i].Name < profiles[j].Name
	})
}

func (m *Manager) Get(id string) *RoutingProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.profiles[id]
}

func (m *Manager) Add(p *RoutingProfile) error {
	if p.ID == "" {
		b := make([]byte, 8)
		rand.Read(b)
		p.ID = hex.EncodeToString(b)
	}
	m.mu.Lock()
	m.profiles[p.ID] = p
	m.mu.Unlock()
	return saveToDisk()
}

func (m *Manager) Update(p *RoutingProfile) error {
	if p.ID == "" {
		p.ID = DefaultRoutingID
	}
	m.mu.Lock()
	m.profiles[p.ID] = p
	m.mu.Unlock()
	return saveToDisk()
}

func (m *Manager) Delete(id string) error {
	if id == DefaultRoutingID {
		return nil
	}
	m.mu.Lock()
	delete(m.profiles, id)
	m.mu.Unlock()
	return saveToDisk()
}

// GetEffective returns the profile by id, or the default if not found.
func (m *Manager) GetEffective(id string) *RoutingProfile {
	if id != "" {
		if p := m.Get(id); p != nil {
			return p
		}
	}
	if p := m.Get(DefaultRoutingID); p != nil {
		return p
	}
	return DefaultRouting()
}

// --- Default ---

func DefaultRouting() *RoutingProfile {
	return &RoutingProfile{
		ID:   DefaultRoutingID,
		Name: "默认分流规则",
		Rules: []RuleItem{
			{Name: "广告拦截", GFW: false, ExtraProxies: []string{"REJECT"}, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/AdvertisingLite/AdvertisingLite_Classical.yaml",
			}},
			{Name: "linux.do", GFW: true, Payload: []string{
				"DOMAIN-SUFFIX,linux.do",
				"DOMAIN-SUFFIX,idcflare.com",
			}},
			{Name: "GitHub", GFW: false, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/GitHub/GitHub.yaml",
			}},
			{Name: "YouTube", GFW: true, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/YouTube/YouTube.yaml",
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/YouTubeMusic/YouTubeMusic.yaml",
			}},
			{Name: "Google", GFW: true, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Google/Google.yaml",
			}},
			{Name: "openAi", GFW: true, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/OpenAI/OpenAI.yaml",
			}},
			{Name: "Netflix", GFW: true, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Netflix/Netflix.yaml",
			}},
			{Name: "Twitter", GFW: true, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Twitter/Twitter.yaml",
			}},
			{Name: "TikTok", GFW: true, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/TikTok/TikTok.yaml",
			}},
			{Name: "Facebook", GFW: true, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Facebook/Facebook.yaml",
			}},
			{Name: "OneDrive", GFW: false, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/OneDrive/OneDrive.yaml",
			}},
			{Name: "Microsoft", GFW: false, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Microsoft/Microsoft.yaml",
			}},
			{Name: "Steam", GFW: false, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@release/rule/Clash/Steam/Steam.yaml",
			}},
			{Name: "Cloudflare", GFW: false, Urls: []string{
				"https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Cloudflare/Cloudflare.yaml",
			}},
		},
	}
}

// EnsureDefault ensures the built-in editable default routing profile exists.
func EnsureDefault() {
	m := GetManager()
	m.mu.RLock()
	hasDefault := m.profiles[DefaultRoutingID] != nil
	m.mu.RUnlock()
	if !hasDefault {
		_ = m.Add(DefaultRouting())
	}
}
