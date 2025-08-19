package ruleset

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"go-sub/internal/datastore"
	"sort"
	"sync"
)

// Clash 支持的规则类型
var SupportedTypes = []string{
	"DOMAIN",
	"DOMAIN-SUFFIX",
	"DOMAIN-KEYWORD",
	"IP-CIDR",
	"IP-CIDR6",
	"SRC-IP-CIDR",
	"GEOIP",
	"DST-PORT",
	"SRC-PORT",
	"PROCESS-NAME",
}

// RuleEntry 定义单条规则
type RuleEntry struct {
	Type  string `json:"type"`  // DOMAIN-SUFFIX, IP-CIDR 等
	Value string `json:"value"` // zoom.us, 192.168.0.0/16 等
}

// RuleSet 定义一个规则集合
type RuleSet struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Rules       []RuleEntry `json:"rules"`
}

// Manager 管理所有规则集
type Manager struct {
	sets map[string]*RuleSet
	mu   sync.RWMutex
}

var globalManager = &Manager{
	sets: make(map[string]*RuleSet),
}

func GetManager() *Manager { return globalManager }

// LoadFromConfig 从磁盘加载
func LoadFromConfig() {
	var sets []*RuleSet
	if err := datastore.ReadJSON("rulesets.json", &sets); err != nil {
		return
	}
	m := GetManager()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range sets {
		m.sets[s.ID] = s
	}
}

func saveToDisk() error {
	m := GetManager()
	m.mu.RLock()
	list := make([]*RuleSet, 0, len(m.sets))
	for _, s := range m.sets {
		list = append(list, s)
	}
	m.mu.RUnlock()
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return datastore.Save("rulesets.json", list)
}

// List 返回所有规则集
func (m *Manager) List() []*RuleSet {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*RuleSet, 0, len(m.sets))
	for _, s := range m.sets {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get 按 ID 获取
func (m *Manager) Get(id string) *RuleSet {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sets[id]
}

// Add 新增
func (m *Manager) Add(s *RuleSet) error {
	if s.ID == "" {
		b := make([]byte, 6)
		rand.Read(b)
		s.ID = hex.EncodeToString(b)
	}
	m.mu.Lock()
	m.sets[s.ID] = s
	m.mu.Unlock()
	return saveToDisk()
}

// Update 更新
func (m *Manager) Update(s *RuleSet) error {
	if s.ID == "" {
		return fmt.Errorf("id is required")
	}
	m.mu.Lock()
	m.sets[s.ID] = s
	m.mu.Unlock()
	return saveToDisk()
}

// Delete 删除
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	delete(m.sets, id)
	m.mu.Unlock()
	return saveToDisk()
}

// ToYAML 将规则集转换为 Clash rule-provider YAML 格式
func (rs *RuleSet) ToYAML() string {
	out := "payload:\n"
	for _, r := range rs.Rules {
		if r.Type != "" && r.Value != "" {
			out += fmt.Sprintf("  - %s,%s\n", r.Type, r.Value)
		}
	}
	return out
}

// ServeURL 返回该规则集对外服务的 URL
func (rs *RuleSet) ServeURL(baseURL string) string {
	if baseURL == "" {
		baseURL = "http://127.0.0.1"
	}
	return fmt.Sprintf("%s/rules/%s.yaml", baseURL, rs.ID)
}
