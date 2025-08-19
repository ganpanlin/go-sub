package rule

import (
	"go-sub/internal/appconfig"
	"testing"
)

func makeTestNode(name, typ, server string, port int) map[string]interface{} {
	return map[string]interface{}{
		"name":   name,
		"type":   typ,
		"server": server,
		"port":   port,
	}
}

func TestIncludeFilter(t *testing.T) {
	p := &Profile{
		Include: `(?i)香港|HK|hong\s*kong`,
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("香港 IPLC 01", "vless", "1.1.1.1", 443),
		makeTestNode("HK Premium", "ss", "2.2.2.2", 443),
		makeTestNode("Tokyo Node", "ss", "3.3.3.3", 443),
		makeTestNode("US Node", "vless", "4.4.4.4", 443),
	}

	result := e.Process(nodes)
	if len(result) != 2 {
		t.Fatalf("expected 2 nodes after include filter, got %d", len(result))
	}
}

func TestExcludeFilter(t *testing.T) {
	p := &Profile{
		Exclude: `(?i)过期|公告|套餐|未知`,
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("香港 01", "vless", "1.1.1.1", 443),
		makeTestNode("套餐到期", "vless", "2.2.2.2", 443),
		makeTestNode("未知-1", "ss", "3.3.3.3", 443),
		makeTestNode("线路公告", "vless", "4.4.4.4", 443),
	}

	result := e.Process(nodes)
	if len(result) != 1 {
		t.Fatalf("expected 1 node after exclude, got %d", len(result))
	}
}

func TestTypeFilter(t *testing.T) {
	p := &Profile{
		TypeFilter: "vmess|vless",
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("Node 1", "vless", "1.1.1.1", 443),
		makeTestNode("Node 2", "vmess", "2.2.2.2", 443),
		makeTestNode("Node 3", "ss", "3.3.3.3", 443),
	}

	result := e.Process(nodes)
	if len(result) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result))
	}
}

func TestRenamePattern(t *testing.T) {
	p := &Profile{
		RenamePattern: "{code}_{tag}",
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("🇭🇰 专线-香港-03", "ss", "1.1.1.1", 443),
		makeTestNode("🇯🇵 专线-日本-01", "ss", "2.2.2.2", 443),
	}

	result := e.Process(nodes)
	if len(result) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(result))
	}

	n1 := result[0].(map[string]interface{})["name"].(string)
	if n1 != "HK_专线 03" {
		t.Errorf("expected 'HK_专线 03', got '%s'", n1)
	}

	n2 := result[1].(map[string]interface{})["name"].(string)
	if n2 != "JP_专线 01" {
		t.Errorf("expected 'JP_专线 01', got '%s'", n2)
	}
}

func TestSortByRegion(t *testing.T) {
	p := &Profile{
		SortBy: "region",
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("🇺🇸 美国", "vless", "1.1.1.1", 443),
		makeTestNode("🇩🇪 德国", "vless", "2.2.2.2", 443),
		makeTestNode("🇭🇰 香港", "vless", "3.3.3.3", 443),
	}

	result := e.Process(nodes)
	if len(result) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(result))
	}

	// Should be sorted: DE, HK, US
	codes := []string{}
	for _, r := range result {
		codes = append(codes, r.(map[string]interface{})["name"].(string))
	}
	// Verify order
	if codes[0] != "🇩🇪 德国" || codes[1] != "🇭🇰 香港" || codes[2] != "🇺🇸 美国" {
		t.Errorf("unexpected order: %v", codes)
	}
}

func TestJSTransform(t *testing.T) {
	p := &Profile{
		Script: `
function transform(node) {
    // Only keep nodes with port > 4000
    if (node.port <= 4000) return false;
    // Override name
    node.name = node.code + "_custom_" + node.port;
    return node;
}
`,
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("🇭🇰 香港", "vless", "1.1.1.1", 8443),
		makeTestNode("🇯🇵 日本", "vless", "2.2.2.2", 3000),
	}

	result := e.Process(nodes)
	if len(result) != 1 {
		t.Fatalf("expected 1 node (port 3000 excluded), got %d", len(result))
	}

	name := result[0].(map[string]interface{})["name"].(string)
	if name != "HK_custom_8443" {
		t.Errorf("expected 'HK_custom_8443', got '%s'", name)
	}
}

func TestJSExcludeNodes(t *testing.T) {
	p := &Profile{
		Script: `
function transform(node) {
    // Exclude announcement nodes
    if (node.name.includes("公告") || node.name.includes("到期")) return false;
    return true;
}
`,
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("🇭🇰 香港 01", "vless", "1.1.1.1", 443),
		makeTestNode("线路公告", "vless", "2.2.2.2", 443),
		makeTestNode("套餐到期", "vless", "3.3.3.3", 443),
	}

	result := e.Process(nodes)
	if len(result) != 1 {
		t.Fatalf("expected 1 node, got %d", len(result))
	}
}

func TestOverrides(t *testing.T) {
	p := &Profile{
		Overrides: map[string]interface{}{
			"tls": true,
			"sni": "override.example.com",
		},
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("Node 1", "vless", "1.1.1.1", 443),
	}

	result := e.Process(nodes)
	m := result[0].(map[string]interface{})
	if m["tls"] != true {
		t.Errorf("expected tls=true, got %v", m["tls"])
	}
	if m["sni"] != "override.example.com" {
		t.Errorf("expected sni override, got %v", m["sni"])
	}
}

func TestManagerCRUD(t *testing.T) {
	appconfig.Init("8080", "config.json", t.TempDir(), "frontend", 10, 60, 10)
	InitManager()
	m := GetManager()

	// Create
	p := m.NewProfile("Test Profile")
	if p.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if p.Name != "Test Profile" {
		t.Errorf("expected 'Test Profile', got '%s'", p.Name)
	}

	// Get
	p2 := m.GetProfile(p.ID)
	if p2 == nil || p2.Name != "Test Profile" {
		t.Fatal("failed to get profile")
	}

	// Update
	_, err := m.UpdateProfile(p.ID, map[string]interface{}{
		"include": "香港",
		"exclude": "过期",
	})
	if err != nil {
		t.Fatal(err)
	}
	p3 := m.GetProfile(p.ID)
	if p3.Include != "香港" {
		t.Errorf("expected include '香港', got '%s'", p3.Include)
	}

	// Delete
	err = m.DeleteProfile(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if m.GetProfile(p.ID) != nil {
		t.Fatal("profile should be deleted")
	}

	// Delete non-existent
	err = m.DeleteProfile("nonexistent")
	if err != ErrProfileNotFound {
		t.Errorf("expected ErrProfileNotFound, got %v", err)
	}
}

func TestJSScriptError(t *testing.T) {
	p := &Profile{
		Script: `function transform(node) { undefined.abc.def(); }`,
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("Test", "vless", "1.1.1.1", 443),
	}
	// Should not crash, node should pass through
	result := e.Process(nodes)
	if len(result) != 1 {
		t.Fatalf("expected 1 node (error in script should not exclude), got %d", len(result))
	}
}

func TestInvalidRegex(t *testing.T) {
	p := &Profile{
		Include: "[invalid",
	}
	_, err := NewEngine(p)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestServerFilter(t *testing.T) {
	p := &Profile{
		ServerFilter: "ip",
	}
	e, err := NewEngine(p)
	if err != nil {
		t.Fatal(err)
	}

	nodes := []interface{}{
		makeTestNode("IP Node", "vless", "1.2.3.4", 443),
		makeTestNode("Domain Node", "vless", "example.com", 443),
	}

	result := e.Process(nodes)
	if len(result) != 1 {
		t.Fatalf("expected 1 node (only IP), got %d", len(result))
	}
}
