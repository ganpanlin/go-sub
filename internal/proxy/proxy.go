package proxy

import (
	"fmt"
	"go-sub/pkg/utils"
	"strconv"
)

func DeduplicateProxies(proxies []interface{}) []interface{} {
	uniqueProxies := make([]interface{}, 0)
	seen := make(map[string]bool)

	for _, p := range proxies {
		if pMap, ok := p.(map[string]interface{}); ok {
			// Ensure required fields exist before creating a unique key
			if pMap["type"] == nil || pMap["server"] == nil || pMap["port"] == nil {
				continue
			}
			uniqueKey := fmt.Sprintf("%v:%v:%v", pMap["type"], pMap["server"], pMap["port"])
			if !seen[uniqueKey] {
				seen[uniqueKey] = true
				uniqueProxies = append(uniqueProxies, p)
			}
		}
	}
	return uniqueProxies
}

func ValidateProxies(proxies []interface{}) []interface{} {
	validProxies := make([]interface{}, 0)
	for _, p := range proxies {
		if pMap, ok := p.(map[string]interface{}); ok {
			// Check for presence of essential keys
			if pMap["name"] == nil || pMap["server"] == nil || pMap["type"] == nil || pMap["port"] == nil {
				continue
			}

			// Validate that port is a valid integer
			portStr := utils.SafeToString(pMap["port"])
			if _, err := strconv.Atoi(portStr); err != nil {
				continue // Skip proxy if port is not a valid integer
			}

			validProxies = append(validProxies, p)
		}
	}
	return validProxies
}

func RenameProxies(proxies []interface{}, nameFilter string) []interface{} {
	if nameFilter == "" {
		return proxies
	}

	var renamedProxies []interface{}
	regionCounts := make(map[string]int)

	for _, p := range proxies {
		if pMap, ok := p.(map[string]interface{}); ok {
			server := ""
			if s, ok := pMap["server"]; ok {
				server = fmt.Sprintf("%v", s)
			}
			name := fmt.Sprintf("%v", pMap["name"])

			code := DetectRegion(name, server)
			if code == "" {
				code = "UN"
			}

			regionCounts[code]++
			cleaned := StripRegionKeywords(name)

			// Build new name: CODE_suffix
			if cleaned != "" {
				pMap["name"] = fmt.Sprintf("%s_%s", code, cleaned)
			} else {
				pMap["name"] = fmt.Sprintf("%s_%d", code, regionCounts[code])
			}
			renamedProxies = append(renamedProxies, pMap)
		}
	}
	return renamedProxies
}

func UpdateProxyGroups(proxyGroups []interface{}, validProxyNames []string) []interface{} {
	var updatedGroups []interface{}

	for _, g := range proxyGroups {
		if gMap, ok := g.(map[string]interface{}); ok {
			if gMap["proxies"] != nil {
				var newProxies []string
				if proxies, ok := gMap["proxies"].([]interface{}); ok {
					for _, pName := range proxies {
						if pName.(string) == "DIRECT" || pName.(string) == "REJECT" {
							newProxies = append(newProxies, pName.(string))
						}
					}
				}
				newProxies = append(newProxies, validProxyNames...)
				gMap["proxies"] = newProxies
			}
			updatedGroups = append(updatedGroups, gMap)
		}
	}
	return updatedGroups
}
