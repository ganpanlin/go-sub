package proxy

import (
	"go-sub/internal/geoip"
	"regexp"
	"sort"
	"strings"
)

// RegionShortCode maps Chinese region names to short codes (HK, JP, US, ...).
var RegionShortCode = map[string]string{
	"香港": "HK", "台湾": "TW", "日本": "JP", "韩国": "KR",
	"新加坡": "SG", "美国": "US", "英国": "GB", "德国": "DE",
	"法国": "FR", "印度": "IN", "加拿大": "CA", "澳大利亚": "AU",
	"俄罗斯": "RU", "巴西": "BR", "荷兰": "NL", "菲律宾": "PH",
	"泰国": "TH", "越南": "VN", "马来西亚": "MY", "印尼": "ID",
	"土耳其": "TR", "南非": "ZA", "阿根廷": "AR", "波兰": "PL",
	"乌克兰": "UA", "瑞典": "SE", "瑞士": "CH", "意大利": "IT",
	"西班牙": "ES", "爱尔兰": "IE", "芬兰": "FI", "丹麦": "DK",
	"挪威": "NO", "捷克": "CZ", "罗马尼亚": "RO", "以色列": "IL",
	"阿联酋": "AE", "沙特": "SA", "智利": "CL", "哥伦比亚": "CO",
	"秘鲁": "PE", "墨西哥": "MX", "新西兰": "NZ", "澳门": "MO",
}

// regionKeywords maps short codes to all searchable keywords.
// Includes emoji flags, Chinese names, English names, abbreviations, and city names.
var regionKeywords = map[string][]string{
	"HK": {"香港", "hk", "hongkong", "hong kong", "🇭🇰"},
	"TW": {"台湾", "tw", "taiwan", "🇹🇼"},
	"JP": {"日本", "东京", "大阪", "jp", "japan", "🇯🇵"},
	"KR": {"韩国", "韓國", "首尔", "kr", "korea", "🇰🇷"},
	"SG": {"新加坡", "sg", "singapore", "🇸🇬"},
	"US": {"美国", "洛杉矶", "纽约", "硅谷", "西雅图", "us", "usa", "united states", "america", "🇺🇸"},
	"GB": {"英国", "伦敦", "gb", "uk", "united kingdom", "britain", "🇬🇧"},
	"DE": {"德国", "法兰克福", "deutschland", "de", "germany", "ger", "🇩🇪"},
	"FR": {"法国", "巴黎", "fr", "france", "🇫🇷"},
	"IN": {"印度", "孟买", "in", "india", "🇮🇳"},
	"CA": {"加拿大", "温哥华", "多伦多", "ca", "canada", "🇨🇦"},
	"AU": {"澳大利亚", "悉尼", "墨尔本", "au", "australia", "🇦🇺"},
	"RU": {"俄罗斯", "莫斯科", "ru", "russia", "moscow", "🇷🇺"},
	"BR": {"巴西", "圣保罗", "br", "brazil", "🇧🇷"},
	"NL": {"荷兰", "阿姆斯特丹", "nl", "netherlands", "🇳🇱"},
	"PH": {"菲律宾", "马尼拉", "ph", "philippines", "🇵🇭"},
	"TH": {"泰国", "曼谷", "th", "thailand", "🇹🇭"},
	"VN": {"越南", "胡志明", "vn", "vietnam", "🇻🇳"},
	"MY": {"马来西亚", "my", "malaysia", "🇲🇾"},
	"ID": {"印尼", "雅加达", "id", "indonesia", "🇮🇩"},
	"TR": {"土耳其", "tr", "turkey", "türkiye", "🇹🇷"},
	"ZA": {"南非", "za", "south africa", "🇿🇦"},
	"AR": {"阿根廷", "布宜诺斯", "ar", "argentina", "🇦🇷"},
	"PL": {"波兰", "pl", "poland", "🇵🇱"},
	"UA": {"乌克兰", "ua", "ukraine", "🇺🇦"},
	"SE": {"瑞典", "se", "sweden", "🇸🇪"},
	"CH": {"瑞士", "ch", "switzerland", "🇨🇭"},
	"IT": {"意大利", "米兰", "it", "italy", "🇮🇹"},
	"ES": {"西班牙", "马德里", "es", "spain", "🇪🇸"},
	"IE": {"爱尔兰", "都柏林", "ie", "ireland", "🇮🇪"},
	"FI": {"芬兰", "赫尔辛基", "fi", "finland", "🇫🇮"},
	"DK": {"丹麦", "dk", "denmark", "🇩🇰"},
	"NO": {"挪威", "no", "norway", "🇳🇴"},
	"CZ": {"捷克", "cz", "czech", "🇨🇿"},
	"RO": {"罗马尼亚", "ro", "romania", "🇷🇴"},
	"IL": {"以色列", "il", "israel", "🇮🇱"},
	"AE": {"阿联酋", "迪拜", "ae", "uae", "🇦🇪"},
	"SA": {"沙特", "sa", "saudi", "🇸🇦"},
	"CL": {"智利", "cl", "chile", "🇨🇱"},
	"CO": {"哥伦比亚", "co", "colombia", "🇨🇴"},
	"PE": {"秘鲁", "pe", "peru", "🇵🇪"},
	"MX": {"墨西哥", "mx", "mexico", "🇲🇽"},
	"NZ": {"新西兰", "nz", "new zealand", "🇳🇿"},
	"MO": {"澳门", "mo", "macau", "macao", "🇲🇴"},
}

// allKeywordsFlat is a sorted list of all keywords, longest first, for efficient stripping.
var allKeywordsFlat []string

func init() {
	seen := make(map[string]bool)
	for _, keywords := range regionKeywords {
		for _, kw := range keywords {
			if !seen[kw] {
				seen[kw] = true
				allKeywordsFlat = append(allKeywordsFlat, kw)
			}
		}
	}
	sort.Slice(allKeywordsFlat, func(i, j int) bool {
		return len([]rune(allKeywordsFlat[i])) > len([]rune(allKeywordsFlat[j]))
	})
}

// ExpandRegionNameFilter expands a user-provided region name filter into a regex
// that matches all known aliases for that region.
func ExpandRegionNameFilter(nameFilter string) string {
	if nameFilter == "" {
		return ".*"
	}

	filterLower := strings.ToLower(nameFilter)

	// Check if it's a short code directly
	if keywords, ok := regionKeywords[strings.ToUpper(nameFilter)]; ok {
		return `(?i)` + strings.Join(keywords, "|")
	}

	// Check if any known keyword is contained in the filter
	for code, keywords := range regionKeywords {
		for _, kw := range keywords {
			if matchKeyword(filterLower, kw) {
				return `(?i)` + strings.Join(keywords, "|")
			}
		}
		// Also check if the Chinese region name matches
		for cnName := range RegionShortCode {
			if code == getCodeForRegion(cnName) {
				if matchKeyword(filterLower, cnName) {
					return `(?i)` + strings.Join(keywords, "|")
				}
			}
		}
	}

	return `(?i)` + nameFilter
}

// DetectRegion tries to detect region from node name, then falls back to IP GeoIP.
// Returns the short code (e.g., "HK", "JP", "US").
func DetectRegion(nodeName, server string) string {
	if code := detectRegionFromName(nodeName); code != "" {
		return code
	}
	if server != "" {
		if region := geoip.LookupRegion(server); region != "" {
			if code, ok := RegionShortCode[region]; ok {
				return code
			}
			return region
		}
	}
	return ""
}

// detectRegionFromName searches node name for known region keywords.
// Returns the short code or empty string.
func detectRegionFromName(nodeName string) string {
	nameLower := strings.ToLower(nodeName)
	for code, keywords := range regionKeywords {
		for _, kw := range keywords {
			if matchKeyword(nameLower, kw) {
				return code
			}
		}
	}
	return ""
}

// StripRegionKeywords removes all known region keywords from a node name,
// returning a cleaned name suitable for use as a suffix in renamed nodes.
func StripRegionKeywords(nodeName string) string {
	// Collect all keywords, sorted longest first for greedy matching.
	// Remove them iteratively using the matchKeyword logic.
	// We need to track which character positions have been "consumed" by a match.
	runes := []rune(nodeName)
	removed := make([]bool, len(runes))

	for _, kw := range allKeywordsFlat {
		kwRunes := []rune(kw)
		kwLower := strings.ToLower(kw)

		for pos := 0; pos <= len(runes)-len(kwRunes); pos++ {
			// Skip if any rune in this range is already removed
			allAvailable := true
			for i := 0; i < len(kwRunes); i++ {
				if removed[pos+i] {
					allAvailable = false
					break
				}
			}
			if !allAvailable {
				continue
			}

			// Check if keyword matches at this position
			segment := strings.ToLower(string(runes[pos : pos+len(kwRunes)]))
			if segment != kwLower {
				continue
			}

			// Check word boundary (same logic as matchKeyword, but using rune positions)
			hasNonASCII := false
			for _, r := range kwLower {
				if r > 127 {
					hasNonASCII = true
					break
				}
			}

			if !hasNonASCII {
				beforeOK := pos == 0 || !isLetterOrDigit(runes[pos-1])
				afterOK := pos+len(kwRunes) >= len(runes) || !isLetterOrDigit(runes[pos+len(kwRunes)])
				if !(beforeOK && afterOK) {
					continue
				}
			}

			// Mark these runes as removed
			for i := 0; i < len(kwRunes); i++ {
				removed[pos+i] = true
			}
		}
	}

	// Rebuild string from non-removed runes
	var result []rune
	for i, r := range runes {
		if !removed[i] {
			result = append(result, r)
		}
	}

	// Normalize whitespace/separators
	cleaned := regexp.MustCompile(`[\s_\-+|:：]+`).ReplaceAllString(string(result), " ")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// matchKeyword checks if keyword appears in text at a word boundary.
// For ASCII keywords, it requires non-alphanumeric before/after.
// For non-ASCII (Chinese etc.), it does substring matching since Chinese doesn't have word boundaries.
func matchKeyword(text, keyword string) bool {
	textLower := strings.ToLower(text)
	kwLower := strings.ToLower(keyword)

	// If keyword contains non-ASCII characters, use simple substring matching
	for _, r := range kwLower {
		if r > 127 {
			return strings.Contains(textLower, kwLower)
		}
	}

	idx := strings.Index(textLower, kwLower)
	for idx != -1 {
		beforeOK := idx == 0 || !isLetterOrDigit(rune(textLower[idx-1]))
		afterIdx := idx + len(kwLower)
		afterOK := afterIdx >= len(textLower) || !isLetterOrDigit(rune(textLower[afterIdx]))

		if beforeOK && afterOK {
			return true
		}
		nextIdx := strings.Index(textLower[idx+1:], kwLower)
		if nextIdx == -1 {
			break
		}
		idx = idx + 1 + nextIdx
	}
	return false
}

func isLetterOrDigit(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func getCodeForRegion(region string) string {
	if code, ok := RegionShortCode[region]; ok {
		return code
	}
	return ""
}

// GetRegionFromIP resolves an IP or domain to a Chinese region name via GeoIP.
func GetRegionFromIP(host string) string {
	return geoip.LookupRegion(host)
}

// getRegionFromNode is the legacy helper used by RenameProxies.
func getRegionFromNode(nodeName, server string) string {
	code := DetectRegion(nodeName, server)
	if code == "" {
		// Last resort: extract prefix from node name
		parts := regexp.MustCompile(`[\s_\-+|:：]`).Split(nodeName, -1)
		if len(parts) > 0 {
			return parts[0]
		}
		return "node"
	}
	return code
}
