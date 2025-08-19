package geoip

import (
	"encoding/json"
	"fmt"
	"go-sub/internal/cache"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RegionCode maps country codes to Chinese region names.
var RegionCode = map[string]string{
	"HK": "香港", "TW": "台湾", "JP": "日本", "KR": "韩国",
	"SG": "新加坡", "US": "美国", "GB": "英国", "DE": "德国",
	"FR": "法国", "IN": "印度", "CA": "加拿大", "AU": "澳大利亚",
	"RU": "俄罗斯", "BR": "巴西", "NL": "荷兰", "PH": "菲律宾",
	"TH": "泰国", "VN": "越南", "MY": "马来西亚", "ID": "印尼",
	"TR": "土耳其", "ZA": "南非", "AR": "阿根廷", "PL": "波兰",
	"UA": "乌克兰", "SE": "瑞典", "CH": "瑞士", "IT": "意大利",
	"ES": "西班牙", "IE": "爱尔兰", "FI": "芬兰", "DK": "丹麦",
	"NO": "挪威", "CZ": "捷克", "RO": "罗马尼亚", "IL": "以色列",
	"AE": "阿联酋", "SA": "沙特", "CL": "智利", "CO": "哥伦比亚",
	"PE": "秘鲁", "MX": "墨西哥", "NZ": "新西兰", "MO": "澳门",
}

type ipAPIResponse struct {
	CountryCode string `json:"countryCode"`
	Status      string `json:"status"`
}

// LookupRegion resolves an IP or domain to a Chinese region name.
// Returns empty string if lookup fails.
func LookupRegion(host string) string {
	// 1. Try cache first
	cacheKey := "geoip:" + host
	if cached, found := cache.Get(cacheKey); found {
		if region, ok := cached.(string); ok {
			return region
		}
	}

	// 2. Resolve domain to IP if needed
	ip := host
	if !isIPAddress(host) {
		resolved, err := resolveHost(host)
		if err != nil || resolved == "" {
			return ""
		}
		ip = resolved
	}

	// 3. Query ip-api.com (free, no API key, 45 req/min)
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,countryCode", ip)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var result ipAPIResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return ""
	}
	if result.Status != "success" {
		return ""
	}

	region := RegionCode[result.CountryCode]
	if region == "" {
		region = result.CountryCode
	}

	// 4. Cache for 24 hours
	cache.Set(cacheKey, region, 24*time.Hour)

	return region
}

// BatchLookupRegions resolves multiple hosts concurrently with deduplication.
// Returns a map[host]regionName.
func BatchLookupRegions(hosts []string) map[string]string {
	results := make(map[string]string)
	unique := make(map[string]bool)
	var toLookup []string

	for _, h := range hosts {
		if !unique[h] {
			unique[h] = true
			// Check cache first
			cacheKey := "geoip:" + h
			if cached, found := cache.Get(cacheKey); found {
				if region, ok := cached.(string); ok {
					results[h] = region
					continue
				}
			}
			if isIPAddress(h) {
				toLookup = append(toLookup, h)
			} else {
				toLookup = append(toLookup, h)
			}
		}
	}

	type kv struct {
		host   string
		region string
	}

	ch := make(chan kv, len(toLookup))
	sem := make(chan struct{}, 10) // Limit concurrency to 10

	for _, h := range toLookup {
		go func(host string) {
			sem <- struct{}{}
			defer func() { <-sem }()
			region := LookupRegion(host)
			ch <- kv{host: host, region: region}
		}(h)
	}

	for i := 0; i < len(toLookup); i++ {
		r := <-ch
		if r.region != "" {
			results[r.host] = r.region
		}
	}

	return results
}

func isIPAddress(str string) bool {
	parts := strings.Split(str, ":")
	host := parts[0]
	p := strings.Split(host, ".")
	if len(p) != 4 {
		return false
	}
	for _, seg := range p {
		n, err := strconv.Atoi(seg)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	return true
}

func resolveHost(host string) (string, error) {
	// Try resolving domain to IP via public DNS API
	url := fmt.Sprintf("https://dns.google/resolve?name=%s&type=A", host)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var dnsResp struct {
		Answer []struct {
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.Unmarshal(body, &dnsResp); err != nil || len(dnsResp.Answer) == 0 {
		return "", fmt.Errorf("cannot resolve %s", host)
	}
	return dnsResp.Answer[0].Data, nil
}
