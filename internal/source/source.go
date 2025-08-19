package source

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"go-sub/internal/datastore"
	"os"
	"strings"
)

const (
	TypeRemoteURL = "remote_url"
	TypeLocalURI  = "local_uri"
	TypeLocalYAML = "local_yaml"
)

// Config is the persisted source metadata in data/sources.json.
type Config struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url,omitempty"`
	Content string `json:"content,omitempty"`
	UA      string `json:"ua,omitempty"`
	Enabled bool   `json:"enabled"`
}

type persistedConfig struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url,omitempty"`
	Content string `json:"content,omitempty"`
	UA      string `json:"ua,omitempty"`
	Enabled *bool  `json:"enabled,omitempty"`
}

func LoadAll() ([]Config, error) {
	var persisted []persistedConfig
	if err := datastore.ReadJSON("sources.json", &persisted); err != nil {
		if os.IsNotExist(err) {
			return []Config{}, nil
		}
		return nil, err
	}

	sources := make([]Config, 0, len(persisted))
	for _, p := range persisted {
		cfg := Config{
			ID:      p.ID,
			Name:    p.Name,
			Type:    p.Type,
			URL:     p.URL,
			Content: p.Content,
			UA:      p.UA,
			Enabled: true,
		}
		if p.Enabled != nil {
			cfg.Enabled = *p.Enabled
		}
		Normalize(&cfg)
		sources = append(sources, cfg)
	}
	return sources, nil
}

func SaveAll(sources []Config) error {
	for i := range sources {
		Normalize(&sources[i])
	}
	return datastore.Save("sources.json", sources)
}

func Normalize(cfg *Config) {
	if cfg.Type == "" {
		switch {
		case cfg.Content != "":
			cfg.Type = TypeLocalURI
		case strings.HasPrefix(cfg.URL, "data:"):
			cfg.Type = TypeLocalURI
		default:
			cfg.Type = TypeRemoteURL
		}
	}
	if cfg.ID == "" {
		cfg.ID = GenerateID(*cfg)
	}
	if cfg.Name == "" {
		if cfg.URL != "" {
			cfg.Name = cfg.URL
		} else {
			cfg.Name = "本地订阅"
		}
	}
}

func RuntimeURL(cfg Config) string {
	if cfg.Content != "" {
		return "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte(cfg.Content))
	}
	return cfg.URL
}

func EnabledRuntimeURLs(refs []string) []string {
	sources, err := LoadAll()
	if err != nil {
		return refs
	}

	if len(refs) == 0 {
		urls := make([]string, 0, len(sources))
		for _, cfg := range sources {
			if cfg.Enabled {
				urls = append(urls, RuntimeURL(cfg))
			}
		}
		return urls
	}

	urls := make([]string, 0, len(refs))
	for _, ref := range refs {
		if cfg, ok := FindByRef(sources, ref); ok {
			if cfg.Enabled {
				urls = append(urls, RuntimeURL(cfg))
			}
			continue
		}
		urls = append(urls, ref)
	}
	return urls
}

func NameMap() map[string]string {
	sources, err := LoadAll()
	if err != nil {
		return map[string]string{}
	}

	names := make(map[string]string, len(sources)*3)
	for _, cfg := range sources {
		runtimeURL := RuntimeURL(cfg)
		names[cfg.ID] = cfg.Name
		names[cfg.URL] = cfg.Name
		names[runtimeURL] = cfg.Name
	}
	return names
}

func UAForRuntimeURL(runtimeURL string) string {
	sources, err := LoadAll()
	if err != nil {
		return ""
	}
	for _, cfg := range sources {
		if RuntimeURL(cfg) == runtimeURL {
			return cfg.UA
		}
	}
	return ""
}

func FindByRef(sources []Config, ref string) (Config, bool) {
	for _, cfg := range sources {
		if cfg.ID == ref || cfg.URL == ref || RuntimeURL(cfg) == ref {
			return cfg, true
		}
	}
	return Config{}, false
}

func FindByID(sources []Config, id string) (Config, bool) {
	for _, cfg := range sources {
		if cfg.ID == id {
			return cfg, true
		}
	}
	return Config{}, false
}

func GenerateID(cfg Config) string {
	key := cfg.URL
	if key == "" {
		key = cfg.Name + "\n" + cfg.Content
	}
	if key != "\n" && key != "" {
		sum := sha256.Sum256([]byte(key))
		return hex.EncodeToString(sum[:8])
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
