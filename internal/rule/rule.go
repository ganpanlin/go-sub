package rule

import (
	"crypto/rand"
	"encoding/hex"
	"go-sub/internal/datastore"
	"regexp"
	"sync"
	"time"
)

// Profile is a saved filter/rule configuration.
type Profile struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Sources: specific URLs to use (empty = use all cached sources)
	Sources []string `json:"sources,omitempty"`
	// Routing profile to use for proxy-groups/rules generation
	RoutingID string `json:"routing_id,omitempty"`
	// Filter rules
	Include      string `json:"include,omitempty"`       // regex: keep nodes matching name
	Exclude      string `json:"exclude,omitempty"`       // regex: remove nodes matching name
	TypeFilter   string `json:"type_filter,omitempty"`   // node type filter (vmess|ss|trojan|...)
	ServerFilter string `json:"server_filter,omitempty"` // ip|domain|regex
	// JS transform script
	Script string `json:"script,omitempty"` // JS code with optional transform(node) function
	// Field overrides
	Overrides map[string]interface{} `json:"overrides,omitempty"` // field-level overrides
	// Rename
	RenamePattern string `json:"rename_pattern,omitempty"` // e.g., "{code}_{tag}"
	// Source prefix: prepend source name to node names
	SourcePrefix string `json:"source_prefix,omitempty"` // "off" | "name" | "domain" (default: off)
	// Sort
	SortBy string `json:"sort_by,omitempty"` // region|name|type|none (default: none)
}

// Manager manages profiles with thread-safe access.
type Manager struct {
	profiles map[string]*Profile
	mu       sync.RWMutex
}

var globalManager *Manager

// InitManager initializes the global profile manager.
func InitManager() {
	globalManager = &Manager{
		profiles: make(map[string]*Profile),
	}
	_ = globalManager.Load()
}

// GetManager returns the global manager.
func GetManager() *Manager {
	return globalManager
}

// NewProfile creates a new profile with a generated ID.
func (m *Manager) NewProfile(name string) *Profile {
	p := &Profile{
		ID:        generateID(),
		Token:     generateToken(),
		Name:      name,
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.mu.Lock()
	m.profiles[p.ID] = p
	m.mu.Unlock()
	_ = m.Save()
	return p
}

// GetProfile returns a profile by ID.
func (m *Manager) GetProfile(id string) *Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.profiles[id]
}

// GetProfileByToken returns a profile by share token.
func (m *Manager) GetProfileByToken(token string) *Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.profiles {
		if p.Token == token {
			return p
		}
	}
	return nil
}

// GetAllProfiles returns all profiles.
func (m *Manager) GetAllProfiles() []*Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Profile, 0, len(m.profiles))
	for _, p := range m.profiles {
		result = append(result, p)
	}
	return result
}

// UpdateProfile updates a profile's fields.
func (m *Manager) UpdateProfile(id string, updates map[string]interface{}) (*Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.profiles[id]
	if !ok {
		return nil, ErrProfileNotFound
	}

	if name, ok := updates["name"].(string); ok && name != "" {
		p.Name = name
	}
	if include, ok := updates["include"].(string); ok {
		p.Include = include
	}
	if exclude, ok := updates["exclude"].(string); ok {
		p.Exclude = exclude
	}
	if tf, ok := updates["type_filter"].(string); ok {
		p.TypeFilter = tf
	}
	if sf, ok := updates["server_filter"].(string); ok {
		p.ServerFilter = sf
	}
	if script, ok := updates["script"].(string); ok {
		p.Script = script
	}
	if overrides, ok := updates["overrides"].(map[string]interface{}); ok {
		p.Overrides = overrides
	}
	if rp, ok := updates["rename_pattern"].(string); ok {
		p.RenamePattern = rp
	}
	if sb, ok := updates["sort_by"].(string); ok {
		p.SortBy = sb
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		p.Enabled = enabled
	}
	if resetToken, ok := updates["reset_token"].(bool); ok && resetToken {
		p.Token = generateToken()
	}
	if sources, ok := updates["sources"]; ok {
		if arr, ok := sources.([]interface{}); ok {
			var urls []string
			for _, u := range arr {
				if s, ok := u.(string); ok && s != "" {
					urls = append(urls, s)
				}
			}
			p.Sources = urls
		}
	}

	if routingID, ok := updates["routing_id"].(string); ok {
		p.RoutingID = routingID
	}
	if sp, ok := updates["source_prefix"].(string); ok {
		p.SourcePrefix = sp
	}
	p.UpdatedAt = time.Now()
	go m.Save()
	return p, nil
}

// DeleteProfile removes a profile.
func (m *Manager) DeleteProfile(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.profiles[id]; !ok {
		return ErrProfileNotFound
	}
	delete(m.profiles, id)
	go m.Save()
	return nil
}

// CompileFilters compiles the include/exclude regex patterns.
func (p *Profile) CompileFilters() (includeRe *regexp.Regexp, excludeRe *regexp.Regexp, err error) {
	if p.Include != "" {
		includeRe, err = regexp.Compile(p.Include)
		if err != nil {
			return nil, nil, err
		}
	}
	if p.Exclude != "" {
		excludeRe, err = regexp.Compile(p.Exclude)
		if err != nil {
			return nil, nil, err
		}
	}
	return
}

// Load loads profiles from data/profiles.json.
func (m *Manager) Load() error {
	var profiles []*Profile
	if err := datastore.ReadJSON("profiles.json", &profiles); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles = make(map[string]*Profile)
	for _, p := range profiles {
		if p.ID != "" {
			// Migration for profiles created before token/enabled fields existed.
			if p.Token == "" {
				p.Token = generateToken()
			}
			if !p.Enabled {
				p.Enabled = true
			}
			m.profiles[p.ID] = p
		}
	}
	return nil
}

// Save saves profiles to data/profiles.json.
func (m *Manager) Save() error {
	m.mu.RLock()
	profiles := make([]*Profile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, p)
	}
	m.mu.RUnlock()

	return datastore.Save("profiles.json", profiles)
}

var ErrProfileNotFound = &ProfileError{"profile not found"}

type ProfileError struct {
	Msg string
}

func (e *ProfileError) Error() string {
	return e.Msg
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	// 16 random bytes = 128-bit share token. Do not derive from profile name.
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
