package converter

import (
	"fmt"
	"net/http"
)

// Converter converts internal proxy maps to a client-specific output format.
type Converter interface {
	// Convert takes processed proxies, proxy-groups, rules, rule-providers, and returns (outputBytes, contentType).
	Convert(proxies []map[string]interface{}, groups []interface{}, ruleProviders map[string]interface{}, rules []string, extra map[string]interface{}) ([]byte, string, error)
	Name() string
}

// Registry holds all registered converters.
var registry = map[string]Converter{}

// Register adds a converter to the registry.
func Register(name string, c Converter) {
	registry[name] = c
}

// Get returns a converter by client type name.
func Get(clientType string) Converter {
	if c, ok := registry[clientType]; ok {
		return c
	}
	return registry["clash"] // default
}

// AvailableTypes returns all registered type names.
func AvailableTypes() []string {
	types := make([]string, 0, len(registry))
	for k := range registry {
		types = append(types, k)
	}
	return types
}

// WriteOutput writes converter output to http.ResponseWriter with appropriate headers.
func WriteOutput(w http.ResponseWriter, data []byte, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=sub")
	w.Write(data)
}

// Helper: safely get a string field from a proxy map.
func strField(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// Helper: safely get an int field from a proxy map.
func intField(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case string:
			var n int
			fmt.Sscanf(val, "%d", &n)
			return n
		}
	}
	return 0
}

// Helper: safely get a bool field from a proxy map.
func boolField(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		case string:
			return val == "true" || val == "1"
		}
	}
	return false
}

// Helper: safely get a map field from a proxy map.
func mapField(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if vm, ok := v.(map[string]interface{}); ok {
			return vm
		}
	}
	return nil
}

// Helper: safely get a string slice field from a proxy map.
func strSliceField(m map[string]interface{}, key string) []string {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case []string:
			return val
		case []interface{}:
			var result []string
			for _, item := range val {
				result = append(result, fmt.Sprintf("%v", item))
			}
			return result
		}
	}
	return nil
}
