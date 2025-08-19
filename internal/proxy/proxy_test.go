package proxy

import (
	"testing"
)

func TestDeduplicateProxies(t *testing.T) {
	proxies := []interface{}{
		map[string]interface{}{"name": "a", "type": "vmess", "server": "1.1.1.1", "port": 443},
		map[string]interface{}{"name": "b", "type": "vmess", "server": "1.1.1.1", "port": 443}, // duplicate
		map[string]interface{}{"name": "c", "type": "vmess", "server": "2.2.2.2", "port": 443},
		map[string]interface{}{"name": "d", "type": "ss", "server": "1.1.1.1", "port": 443}, // different type
	}

	result := DeduplicateProxies(proxies)
	if len(result) != 3 {
		t.Fatalf("expected 3 proxies after dedup, got %d", len(result))
	}
}

func TestValidateProxies(t *testing.T) {
	proxies := []interface{}{
		map[string]interface{}{"name": "valid", "type": "vmess", "server": "1.1.1.1", "port": 443},
		map[string]interface{}{"name": "no-port", "type": "vmess", "server": "1.1.1.1"},                 // missing port
		map[string]interface{}{"name": "bad-port", "type": "vmess", "server": "1.1.1.1", "port": "abc"}, // invalid port
	}

	result := ValidateProxies(proxies)
	if len(result) != 1 {
		t.Fatalf("expected 1 valid proxy, got %d", len(result))
	}
}

func TestRenameProxiesShortCode(t *testing.T) {
	proxies := []interface{}{
		map[string]interface{}{"name": "香港 IPLC 01", "type": "vmess", "server": "1.1.1.1", "port": 443},
		map[string]interface{}{"name": "Hong Kong Premium", "type": "vmess", "server": "2.2.2.2", "port": 443},
		map[string]interface{}{"name": "东京 Node A", "type": "ss", "server": "3.3.3.3", "port": 8388},
		map[string]interface{}{"name": "JP Tokyo 01", "type": "ss", "server": "4.4.4.4", "port": 8388},
		map[string]interface{}{"name": "UnknownNode-XYZ", "type": "trojan", "server": "5.5.5.5", "port": 443},
	}

	result := RenameProxies(proxies, ".*")

	names := make([]string, len(result))
	for i, p := range result {
		names[i] = p.(map[string]interface{})["name"].(string)
	}

	// 香港节点应去掉"香港"和"Hong Kong"关键词
	if names[0] != "HK_IPLC 01" {
		t.Errorf("index 0: expected 'HK_IPLC 01', got '%s'", names[0])
	}
	if names[1] != "HK_Premium" {
		t.Errorf("index 1: expected 'HK_Premium', got '%s'", names[1])
	}
	// 东京节点应去掉"东京"关键词
	if names[2] != "JP_Node A" {
		t.Errorf("index 2: expected 'JP_Node A', got '%s'", names[2])
	}
	// JP in name (JP at word boundary)
	if names[3] != "JP_Tokyo 01" {
		t.Errorf("index 3: expected 'JP_Tokyo 01', got '%s'", names[3])
	}
	// 5.5.5.5 resolves to DE via GeoIP fallback
	if names[4] != "DE_UnknownNode XYZ" && names[4] != "UN_UnknownNode XYZ" {
		t.Logf("index 4: got '%s' (GeoIP dependent)", names[4])
	}
}

func TestRenameProxiesNoFilter(t *testing.T) {
	proxies := []interface{}{
		map[string]interface{}{"name": "a", "type": "vmess", "server": "1.1.1.1", "port": 443},
	}

	result := RenameProxies(proxies, "")
	if len(result) != 1 {
		t.Fatalf("expected no rename when filter is empty")
	}
	if result[0].(map[string]interface{})["name"] != "a" {
		t.Errorf("expected name unchanged, got '%v'", result[0].(map[string]interface{})["name"])
	}
}
