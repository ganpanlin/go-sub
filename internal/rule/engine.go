package rule

import (
	"fmt"
	"go-sub/internal/proxy"
	"log/slog"
	"regexp"
	"sort"
	"strings"

	"github.com/dop251/goja"
)

// Engine executes a profile's rules against a list of proxies.
type Engine struct {
	profile   *Profile
	includeRe *regexp.Regexp
	excludeRe *regexp.Regexp
	typeRe    *regexp.Regexp
	vm        *goja.Runtime
	scriptFn  goja.Callable
}

// NewEngine creates a new rule engine for the given profile.
func NewEngine(p *Profile) (*Engine, error) {
	e := &Engine{profile: p}

	// Compile filters
	includeRe, excludeRe, err := p.CompileFilters()
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}
	e.includeRe = includeRe
	e.excludeRe = excludeRe

	if p.TypeFilter != "" {
		e.typeRe, err = regexp.Compile(p.TypeFilter)
		if err != nil {
			return nil, fmt.Errorf("invalid type_filter regex: %w", err)
		}
	}

	// Compile JS script
	if p.Script != "" {
		e.vm = goja.New()
		_, err = e.vm.RunString(p.Script)
		if err != nil {
			return nil, fmt.Errorf("script compile error: %w", err)
		}
		// Look for transform function
		fn := e.vm.Get("transform")
		if fn != nil && goja.IsUndefined(fn) == false && goja.IsNull(fn) == false {
			if c, ok := goja.AssertFunction(fn); ok {
				e.scriptFn = c
			}
		}
		// Also look for filter function
	}

	return e, nil
}

// Process applies all rules to the proxy list and returns the filtered/transformed list.
func (e *Engine) Process(proxies []interface{}) []interface{} {
	var result []interface{}

	for _, p := range proxies {
		pMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		name := fmt.Sprintf("%v", pMap["name"])
		typ := fmt.Sprintf("%v", pMap["type"])
		server := ""

		if s, ok := pMap["server"]; ok {
			server = fmt.Sprintf("%v", s)
		}

		// 1. Include filter
		if e.includeRe != nil {
			if !e.includeRe.MatchString(name) {
				continue
			}
		}

		// 2. Exclude filter
		if e.excludeRe != nil {
			if e.excludeRe.MatchString(name) {
				continue
			}
		}

		// 3. Type filter
		if e.typeRe != nil {
			if !e.typeRe.MatchString(typ) {
				continue
			}
		}

		// 4. Server filter
		if e.profile.ServerFilter == "ip" {
			if !isIPAddress(server) {
				continue
			}
		} else if e.profile.ServerFilter == "domain" {
			if isIPAddress(server) || server == "" {
				continue
			}
		}

		// 5. JS transform
		if e.scriptFn != nil {
			excluded := e.runTransform(pMap, name, typ, server)
			if excluded {
				continue
			}
		}

		// 6. Overrides
		if len(e.profile.Overrides) > 0 {
			e.applyOverrides(pMap)
		}

		// 7. Rename
		if e.profile.RenamePattern != "" {
			e.applyRename(pMap, name, server)
		}

		result = append(result, pMap)
	}

	// 8. Sort
	result = e.applySort(result)

	return result
}

// runTransform runs the user's JS transform function.
// Returns true if the node should be excluded.
func (e *Engine) runTransform(pMap map[string]interface{}, name, typ, server string) bool {
	if e.scriptFn == nil {
		return false
	}

	// Build node object for JS
	nodeObj := e.vm.NewObject()
	nodeObj.Set("name", name)
	nodeObj.Set("type", typ)
	nodeObj.Set("server", server)
	if v, ok := pMap["port"]; ok {
		nodeObj.Set("port", v)
	}
	if v, ok := pMap["sni"]; ok {
		nodeObj.Set("sni", fmt.Sprintf("%v", v))
	}
	if v, ok := pMap["tls"]; ok {
		nodeObj.Set("tls", v)
	}
	if v, ok := pMap["network"]; ok {
		nodeObj.Set("network", fmt.Sprintf("%v", v))
	}
	if v, ok := pMap["cipher"]; ok {
		nodeObj.Set("cipher", fmt.Sprintf("%v", v))
	}
	// Set region info
	code := proxy.DetectRegion(name, server)
	nodeObj.Set("code", code)
	nodeObj.Set("tag", proxy.StripRegionKeywords(name))

	// Call transform(node)
	ret, err := e.scriptFn(goja.Undefined(), nodeObj)
	if err != nil {
		slog.Error("JS transform error", "error", err)
		return false
	}

	// false/null = exclude
	if goja.IsNull(ret) {
		return true
	}
	// Check boolean: in goja, booleans are represented as value objects
	if b, ok := ret.Export().(bool); ok && !b {
		return true
	}

	// Object = override fields from return value
	if obj := ret.ToObject(e.vm); obj != nil {
		keys := obj.Keys()
		for _, key := range keys {
			val := obj.Get(key)
			// Don't override with undefined
			if !goja.IsUndefined(val) && !goja.IsNull(val) {
				goVal := val.Export()
				pMap[key] = goVal
			}
		}
	}

	return false
}

// applyOverrides applies field-level overrides from the profile.
func (e *Engine) applyOverrides(pMap map[string]interface{}) {
	for key, val := range e.profile.Overrides {
		if key == "name" || key == "server" || key == "port" || key == "sni" ||
			key == "tls" || key == "network" || key == "skip-cert-verify" ||
			key == "uuid" || key == "password" || key == "cipher" ||
			key == "alterId" || key == "flow" || key == "udp" {
			pMap[key] = val
		}
	}
}

// applyRename applies the rename pattern with template variables.
func (e *Engine) applyRename(pMap map[string]interface{}, originalName, server string) {
	name := fmt.Sprintf("%v", pMap["name"])
	typ := fmt.Sprintf("%v", pMap["type"])
	code := proxy.DetectRegion(name, server)
	if code == "" {
		code = "UN"
	}
	tag := proxy.StripRegionKeywords(name)
	if tag == "" {
		tag = "?"
	}

	result := e.profile.RenamePattern
	result = strings.ReplaceAll(result, "{code}", code)
	result = strings.ReplaceAll(result, "{tag}", tag)
	result = strings.ReplaceAll(result, "{type}", typ)
	result = strings.ReplaceAll(result, "{name}", name)
	result = strings.ReplaceAll(result, "{server}", server)
	pMap["name"] = result
}

// applySort sorts the result list.
func (e *Engine) applySort(proxies []interface{}) []interface{} {
	switch e.profile.SortBy {
	case "region":
		sort.SliceStable(proxies, func(i, j int) bool {
			a := proxies[i].(map[string]interface{})
			b := proxies[j].(map[string]interface{})
			codeA := proxy.DetectRegion(fmt.Sprintf("%v", a["name"]), fmt.Sprintf("%v", a["server"]))
			codeB := proxy.DetectRegion(fmt.Sprintf("%v", b["name"]), fmt.Sprintf("%v", b["server"]))
			return codeA < codeB
		})
	case "name":
		sort.SliceStable(proxies, func(i, j int) bool {
			a := proxies[i].(map[string]interface{})
			b := proxies[j].(map[string]interface{})
			return fmt.Sprintf("%v", a["name"]) < fmt.Sprintf("%v", b["name"])
		})
	case "type":
		sort.SliceStable(proxies, func(i, j int) bool {
			a := proxies[i].(map[string]interface{})
			b := proxies[j].(map[string]interface{})
			return fmt.Sprintf("%v", a["type"]) < fmt.Sprintf("%v", b["type"])
		})
	}
	return proxies
}

func isIPAddress(str string) bool {
	parts := strings.SplitN(str, ":", 2)
	host := parts[0]
	p := strings.Split(host, ".")
	if len(p) != 4 {
		return false
	}
	for _, seg := range p {
		n := 0
		for _, c := range seg {
			if c < '0' || c > '9' {
				return false
			}
			n = n*10 + int(c-'0')
		}
		if n < 0 || n > 255 {
			return false
		}
	}
	return true
}
