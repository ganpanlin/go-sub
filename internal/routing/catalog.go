package routing

import (
	"encoding/json"
	"go-sub/internal/appconfig"
	"log"
	"os"
	"sync"
)

// RulePreset 定义一条可勾选的规则预设
type RulePreset struct {
	ID      string   `json:"id"`                // 唯一标识，如 "openai"
	Name    string   `json:"name"`              // 显示名，如 "OpenAI"
	URL     string   `json:"url,omitempty"`     // 单个远程规则 URL
	URLs    []string `json:"urls,omitempty"`    // 多个远程规则 URL（优先于 URL）
	Payload []string `json:"payload,omitempty"` // 内联规则（二选一）
	Default bool     `json:"default"`           // 是否默认选中
}

// RuleCategory 定义一个规则分类
type RuleCategory struct {
	ID    string       `json:"id"`    // 分类标识，如 "ai"
	Name  string       `json:"name"`  // 分类名，如 "🤖 AI服务"
	GFW   bool         `json:"gfw"`   // 该分类是否默认走代理
	Rules []RulePreset `json:"rules"`
}

var (
	catalogMu   sync.RWMutex
	catalogData []RuleCategory
)

func catalogPath() string {
	return appconfig.Get().DataFile("catalog.json")
}

// GetCatalog returns the rule catalog loaded from data/catalog.json.
func GetCatalog() []RuleCategory {
	catalogMu.RLock()
	defer catalogMu.RUnlock()
	if catalogData != nil {
		return catalogData
	}

	path := catalogPath()
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[catalog] failed to read %s: %v, returning empty", path, err)
		return []RuleCategory{}
	}
	if err := json.Unmarshal(data, &catalogData); err != nil {
		log.Printf("[catalog] failed to parse %s: %v", path, err)
		return []RuleCategory{}
	}
	return catalogData
}

// EnsureCatalogDefault writes the built-in catalog to data/catalog.json if it does not exist.
// Called at startup so the rule picker UI always has content.
func EnsureCatalogDefault() {
	path := catalogPath()
	if _, err := os.Stat(path); err == nil {
		return // file exists, nothing to do
	}

	builtIn := defaultCatalog()
	data, err := json.MarshalIndent(builtIn, "", "  ")
	if err != nil {
		log.Printf("[catalog] failed to marshal default: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("[catalog] failed to write default: %v", err)
		return
	}
	log.Printf("[catalog] created default catalog at %s", path)
}

// SaveCatalog saves the catalog to data/catalog.json and updates the cache.
func SaveCatalog(categories []RuleCategory) error {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	data, err := json.MarshalIndent(categories, "", "  ")
	if err != nil {
		return err
	}
	path := catalogPath()
	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	catalogData = categories
	return nil
}

// defaultCatalog returns the built-in rule catalog used when data/catalog.json does not exist.
func defaultCatalog() []RuleCategory {
	cdn := "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/"
	return []RuleCategory{
		{
			ID: "ai", Name: "🤖 AI服务", GFW: true,
			Rules: []RulePreset{
				{ID: "openai", Name: "OpenAI", URLs: []string{cdn + "OpenAI/OpenAI.yaml"}},
				{ID: "claude", Name: "Claude", URLs: []string{cdn + "Claude/Claude.yaml"}},
				{ID: "gemini", Name: "Gemini", URLs: []string{cdn + "Gemini/Gemini.yaml"}},
				{ID: "copilot", Name: "Copilot", URLs: []string{cdn + "Copilot/Copilot.yaml"}},
				{ID: "deepseek", Name: "DeepSeek", URLs: []string{cdn + "DeepSeek/DeepSeek.yaml"}},
			},
		},
		{
			ID: "streaming", Name: "🎬 流媒体", GFW: true,
			Rules: []RulePreset{
				{ID: "youtube", Name: "YouTube", URLs: []string{cdn + "YouTube/YouTube.yaml"}},
				{ID: "netflix", Name: "Netflix", URLs: []string{cdn + "Netflix/Netflix.yaml"}},
				{ID: "disney", Name: "Disney+", URLs: []string{cdn + "Disney/Disney.yaml"}},
				{ID: "hbo", Name: "HBO", URLs: []string{cdn + "HBO/HBO.yaml"}},
				{ID: "spotify", Name: "Spotify", URLs: []string{cdn + "Spotify/Spotify.yaml"}},
				{ID: "tiktok", Name: "TikTok", URLs: []string{cdn + "TikTok/TikTok.yaml"}},
				{ID: "twitch", Name: "Twitch", URLs: []string{cdn + "Twitch/Twitch.yaml"}},
				{ID: "prime_video", Name: "Prime Video", URLs: []string{cdn + "AmazonPrimeVideo/AmazonPrimeVideo.yaml"}},
				{ID: "bilibili_intl", Name: "B站国际版", URLs: []string{cdn + "BiliBiliIntl/BiliBiliIntl.yaml"}},
				{ID: "bahamut", Name: "巴哈姆特", URLs: []string{cdn + "Bahamut/Bahamut.yaml"}},
			},
		},
		{
			ID: "social", Name: "🐦 社交平台", GFW: true,
			Rules: []RulePreset{
				{ID: "twitter", Name: "Twitter / X", URLs: []string{cdn + "Twitter/Twitter.yaml"}},
				{ID: "facebook", Name: "Facebook", URLs: []string{cdn + "Facebook/Facebook.yaml"}},
				{ID: "instagram", Name: "Instagram", URLs: []string{cdn + "Instagram/Instagram.yaml"}},
				{ID: "telegram", Name: "Telegram", URLs: []string{cdn + "Telegram/Telegram.yaml"}},
				{ID: "discord", Name: "Discord", URLs: []string{cdn + "Discord/Discord.yaml"}},
				{ID: "whatsapp", Name: "WhatsApp", URLs: []string{cdn + "WhatsApp/WhatsApp.yaml"}},
				{ID: "reddit", Name: "Reddit", URLs: []string{cdn + "Reddit/Reddit.yaml"}},
				{ID: "line", Name: "Line", URLs: []string{cdn + "Line/Line.yaml"}},
			},
		},
		{
			ID: "game", Name: "🎮 游戏平台", GFW: false,
			Rules: []RulePreset{
				{ID: "steam", Name: "Steam", URLs: []string{cdn + "Steam/Steam.yaml"}},
				{ID: "playstation", Name: "PlayStation", URLs: []string{cdn + "PlayStation/PlayStation.yaml"}},
				{ID: "xbox", Name: "Xbox", URLs: []string{cdn + "Xbox/Xbox.yaml"}},
				{ID: "nintendo", Name: "Nintendo", URLs: []string{cdn + "Nintendo/Nintendo.yaml"}},
				{ID: "blizzard", Name: "暴雪", URLs: []string{cdn + "Blizzard/Blizzard.yaml"}},
				{ID: "epic", Name: "Epic Games", URLs: []string{cdn + "Epic/Epic.yaml"}},
			},
		},
		{
			ID: "dev", Name: "💻 开发工具", GFW: false,
			Rules: []RulePreset{
				{ID: "github", Name: "GitHub", URLs: []string{cdn + "GitHub/GitHub.yaml"}},
				{ID: "gitlab", Name: "GitLab", URLs: []string{cdn + "GitLab/GitLab.yaml"}},
				{ID: "docker", Name: "Docker Hub", URLs: []string{cdn + "Docker/Docker.yaml"}},
				{ID: "jetbrains", Name: "JetBrains", URLs: []string{cdn + "Jetbrains/Jetbrains.yaml"}},
				{ID: "stackoverflow", Name: "Stack Overflow", URLs: []string{cdn + "Stackexchange/Stackexchange.yaml"}},
			},
		},
		{
			ID: "apple", Name: "🍎 苹果服务", GFW: false,
			Rules: []RulePreset{
				{ID: "apple", Name: "Apple 通用", URLs: []string{cdn + "Apple/Apple.yaml"}},
				{ID: "icloud", Name: "iCloud", URLs: []string{cdn + "iCloud/iCloud.yaml"}},
				{ID: "testflight", Name: "TestFlight", URLs: []string{cdn + "TestFlight/TestFlight.yaml"}},
			},
		},
		{
			ID: "microsoft", Name: "Ⓜ️ 微软服务", GFW: false,
			Rules: []RulePreset{
				{ID: "microsoft", Name: "Microsoft 通用", URLs: []string{cdn + "Microsoft/Microsoft.yaml"}},
				{ID: "onedrive", Name: "OneDrive", URLs: []string{cdn + "OneDrive/OneDrive.yaml"}},
				{ID: "teams", Name: "Teams", URLs: []string{cdn + "Teams/Teams.yaml"}},
			},
		},
		{
			ID: "google", Name: "📢 谷歌服务", GFW: true,
			Rules: []RulePreset{
				{ID: "google", Name: "Google 通用", URLs: []string{cdn + "Google/Google.yaml"}},
				{ID: "google_drive", Name: "Google Drive", URLs: []string{cdn + "GoogleDrive/GoogleDrive.yaml"}},
			},
		},
		{
			ID: "privacy", Name: "🔒 隐私安全", GFW: false,
			Rules: []RulePreset{
				{ID: "advertising_lite", Name: "广告拦截 (精简)", URLs: []string{cdn + "AdvertisingLite/AdvertisingLite_Classical.yaml"}},
				{ID: "advertising", Name: "广告拦截 (完整)", URLs: []string{cdn + "Advertising/Advertising.yaml"}},
				{ID: "easyprivacy", Name: "隐私保护", URLs: []string{cdn + "EasyPrivacy/EasyPrivacy.yaml"}},
				{ID: "hijacking", Name: "防劫持", URLs: []string{cdn + "Hijacking/Hijacking.yaml"}},
			},
		},
		{
			ID: "infra", Name: "🌐 基础设施", GFW: false,
			Rules: []RulePreset{
				{ID: "cloudflare", Name: "Cloudflare", URLs: []string{cdn + "Cloudflare/Cloudflare.yaml"}},
				{ID: "global", Name: "国际通用集合", URLs: []string{cdn + "Global/Global.yaml"}},
				{ID: "proxy", Name: "需代理域名", URLs: []string{cdn + "Proxy/Proxy.yaml"}},
			},
		},
		{
			ID: "tools", Name: "🔧 常用工具", GFW: true,
			Rules: []RulePreset{
				{ID: "zoom", Name: "Zoom", Payload: []string{"DOMAIN-SUFFIX,zoom.us", "DOMAIN-SUFFIX,zoomgov.com"}},
				{ID: "notion", Name: "Notion", URLs: []string{cdn + "Notion/Notion.yaml"}},
				{ID: "slack", Name: "Slack", URLs: []string{cdn + "Slack/Slack.yaml"}},
				{ID: "dropbox", Name: "Dropbox", URLs: []string{cdn + "Dropbox/Dropbox.yaml"}},
			},
		},
		{
			ID: "exchange", Name: "💱 交易所", GFW: true,
			Rules: []RulePreset{
				{ID: "binance", Name: "Binance 币安", URLs: []string{cdn + "Binance/Binance.yaml"}},
				{ID: "okx", Name: "OKX 欧易", URLs: []string{cdn + "OKX/OKX.yaml"}},
				{ID: "bybit", Name: "Bybit", Payload: []string{"DOMAIN-SUFFIX,bybit.com"}},
				{ID: "gate", Name: "Gate.io", Payload: []string{"DOMAIN-SUFFIX,gate.io", "DOMAIN-SUFFIX,gatedata.org"}},
				{ID: "bitget", Name: "Bitget", Payload: []string{"DOMAIN-SUFFIX,bitget.com"}},
				{ID: "huobi", Name: "HTX (火币)", Payload: []string{"DOMAIN-SUFFIX,htx.com", "DOMAIN-SUFFIX,huobi.com"}},
				{ID: "coinbase", Name: "Coinbase", Payload: []string{"DOMAIN-SUFFIX,coinbase.com"}},
				{ID: "mexc", Name: "MEXC", Payload: []string{"DOMAIN-SUFFIX,mexc.com"}},
				{ID: "kucoin", Name: "KuCoin", Payload: []string{"DOMAIN-SUFFIX,kucoin.com"}},
			},
		},
	}
}
