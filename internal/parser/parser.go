package parser

import (
	"encoding/base64"
	"gopkg.in/yaml.v3"
	"regexp"
	"strings"
)

func ParseYAML(content []byte) (map[string]interface{}, error) {
	// Preprocess content to remove special tags
	processedContent := preprocessYAMLContent(string(content))

	// Check if the content is Base64 encoded
	if isBase64(processedContent) {
		decoded, err := base64.StdEncoding.DecodeString(processedContent)
		if err == nil {
			processedContent = preprocessYAMLContent(string(decoded))
		}
	}

	// Check for node URIs
	if checkForNodeURIs(processedContent) {
		return parseURIListToConfig(processedContent)
	}

	var config map[string]interface{}
	err := yaml.Unmarshal([]byte(processedContent), &config)
	if err != nil {
		// Content is not valid YAML (e.g. plain text "error" from CDN)
		// Return an empty config with no proxies instead of crashing
		return map[string]interface{}{"proxies": []interface{}{}}, nil
	}
	if config == nil {
		config = map[string]interface{}{"proxies": []interface{}{}}
	}
	return config, nil
}

func preprocessYAMLContent(content string) string {
	// Remove special YAML tags like !<str>
	re := regexp.MustCompile(`!<[^>]+>\s+`)
	content = re.ReplaceAllString(content, "")
	return content
}

func isBase64(str string) bool {
	// Basic check for Base64 characters
	if len(str)%4 != 0 {
		return false
	}
	// Check if the string contains only Base64 valid characters
	matched, _ := regexp.MatchString(`^[A-Za-z0-9+/=\s]+$`, str)
	return matched
}

func checkForNodeURIs(content string) bool {
	prefixes := []string{"vmess://", "ss://", "ssr://", "trojan://", "hysteria2://", "vless://"}
	for _, prefix := range prefixes {
		if strings.Contains(content, prefix) {
			return true
		}
	}
	return false
}

func IsIPAddress(str string) bool {
	matched, _ := regexp.MatchString(`^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$`, str)
	return matched
}

func IsDomainName(str string) bool {
	if IsIPAddress(str) {
		return false
	}
	matched, _ := regexp.MatchString(`^([a-zA-Z0-9_]([a-zA-Z0-9\-_]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z0-9\-]{2,}$`, str)
	return matched
}
