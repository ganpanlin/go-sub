package handler

import (
	"encoding/json"
	"go-sub/internal/ruleset"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// RuleSetListHandler 列出所有规则集
func RuleSetListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	list := ruleset.GetManager().List()
	// 附加服务 URL
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host
	type item struct {
		ruleset.RuleSet
		ServeURL string `json:"serve_url"`
	}
	result := make([]item, 0, len(list))
	for _, s := range list {
		result = append(result, item{RuleSet: *s, ServeURL: s.ServeURL(baseURL)})
	}
	json.NewEncoder(w).Encode(result)
}

// RuleSetAddHandler 新增规则集
func RuleSetAddHandler(w http.ResponseWriter, r *http.Request) {
	var s ruleset.RuleSet
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if s.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if err := ruleset.GetManager().Add(&s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(s)
}

// RuleSetUpdateHandler 更新规则集
func RuleSetUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}
	var s ruleset.RuleSet
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	s.ID = id
	if err := ruleset.GetManager().Update(&s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s)
}

// RuleSetDeleteHandler 删除规则集
func RuleSetDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}
	if err := ruleset.GetManager().Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RuleSetTypesHandler 返回支持的规则类型列表
func RuleSetTypesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ruleset.SupportedTypes)
}

// RuleSetServeHandler 对外提供 YAML 规则文件（无需鉴权，给 Clash rule-provider 用）
func RuleSetServeHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := strings.TrimSuffix(vars["id"], ".yaml")
	rs := ruleset.GetManager().Get(id)
	if rs == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Write([]byte(rs.ToYAML()))
}
