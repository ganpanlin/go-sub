# blackmatrix7/ios_rule_script 规则分类整理

> 来源：https://github.com/blackmatrix7/ios_rule_script/tree/master/rule/Clash
> CDN 前缀：`https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/`

---

## ✅ 系统已集成（默认分流规则）

| 策略组 | 规则 |
|--------|------|
| 🛑 广告拦截 | AdvertisingLite |
| 🤖 AI服务 | 内联规则 (OpenAI/Claude/Gemini/Copilot/DeepSeek) |
| 📢 谷歌服务 | Google |
| 🎬 流媒体 | YouTube, YouTubeMusic, Netflix, Spotify, TikTok, Disney |
| 🐦 社交平台 | Twitter, Facebook, Instagram, Telegram, Discord |
| 💻 GitHub | GitHub |
| 🍎 苹果服务 | Apple |
| Ⓜ️ 微软服务 | Microsoft |
| 🎮 游戏平台 | Steam |
| ☁️ OneDrive | OneDrive |
| 🌐 Cloudflare | Cloudflare |

---

## 📦 可扩展分类（按需添加）

### 🇨🇳 国内常用（直连）

| 名称 | 说明 |
|------|------|
| **China** | 国内通用大集合（推荐） |
| **ChinaMax** | 国内最大化集合 |
| **ChinaMedia** | 国内媒体 |
| **ChinaIPs** / **ChinaIPsBGP** | 中国 IP 段 |
| **Direct** | 直连域名 |

#### 国内社交 / 内容
| 名称 | 说明 |
|------|------|
| WeChat | 微信 |
| Weibo | 微博 |
| DouYin | 抖音 |
| BiliBili | B站 |
| Zhihu | 知乎 |
| XiaoHongShu | 小红书 |
| DouBan | 豆瓣 |
| BaiDuTieBa | 百度贴吧 |
| NGA | NGA论坛 |
| TianYaForum | 天涯论坛 |
| Hupu | 虎扑 |
| Kuan (Coolapk) | 酷安 |

#### 国内视频 / 音乐
| 名称 | 说明 |
|------|------|
| iQIYI | 爱奇艺 |
| Youku | 优酷 |
| TencentVideo | 腾讯视频 |
| Sohu | 搜狐视频 |
| LeTV | 乐视 |
| Migu | 咪咕 |
| NetEaseMusic | 网易云音乐 |
| QQMusic | QQ音乐（Tencent内） |
| KugouKuwo | 酷狗酷吾 |

#### 国内购物 / 生活
| 名称 | 说明 |
|------|------|
| JingDong | 京东 |
| Taobao | 淘宝（Alibaba内） |
| Pinduoduo | 拼多多 |
| MeiTuan | 美团 |
| Eleme | 饿了么 |
| DangDang | 当当 |
| SMZDM | 什么值得买 |
| VipShop | 唯品会 |
| XianYu | 闲鱼 |

#### 国内工具 / 效率
| 名称 | 说明 |
|------|------|
| Baidu | 百度 |
| DingTalk | 钉钉 |
| FeiZhu | 飞猪 |
| GaoDe | 高德地图 |
| DiDi | 滴滴 |
| CSDN | CSDN |
| JueJin | 掘金 |
| JianShu | 简书 |
| CNKI | 知网 |

#### 国内金融
| 名称 | 说明 |
|------|------|
| AliPay | 支付宝 |
| UnionPay | 银联 |
| ICBC | 工商银行 |
| CCB | 建设银行 |
| CMB | 招商银行 |
| PingAn | 平安 |
| EastMoney | 东方财富 |

---

### 🌍 国际通用（代理 / GFW）

| 名称 | 说明 |
|------|------|
| **Global** | 国际通用大集合（推荐） |
| **Proxy** | 需代理域名 |
| **ProxyLite** | 轻量代理列表 |

---

### 🤖 AI 服务（代理）

| 名称 | 说明 |
|------|------|
| OpenAI | ChatGPT / API |
| Claude | Anthropic Claude |
| BardAI | Google Bard |
| Gemini | Google Gemini |
| Copilot | Microsoft Copilot |
| DeepSeek | 已内联，可加远程 |

> 💡 当前用内联规则，如需更全可用远程规则集

---

### 🎬 流媒体 / 视频（代理）

| 名称 | 说明 |
|------|------|
| YouTube | YouTube |
| YouTubeMusic | YouTube Music |
| Netflix | Netflix |
| Disney | Disney+ |
| HBO / HBOAsia / HBOHK / HBOUSA | HBO 各区 |
| Hulu / HuluJP / HuluUSA | Hulu 各区 |
| AmazonPrimeVideo | Amazon Prime |
| Spotify | Spotify |
| TikTok | TikTok |
| Bahamut | 巴哈姆特动画疯 |
| BiliBiliIntl | B站国际版 |
| DAZN | DAZN 体育 |
| DiscoveryPlus | Discovery+ |
| FuboTV | FuboTV |
| Twitch | Twitch |
| Niconico | N站 |
| Emby | Emby 媒体 |
| Plex | Plex 媒体 |
| ParamountPlus | Paramount+ |
| Peacock | Peacock |

#### 国内流媒体
| 名称 | 说明 |
|------|------|
| BiliBili | B站 |
| iQIYI | 爱奇艺 |
| TencentVideo | 腾讯视频 |
| Youku | 优酷 |
| Migu | 咪咕 |

---

### 🐦 社交平台（代理）

| 名称 | 说明 |
|------|------|
| Twitter | X/Twitter |
| Facebook | Facebook |
| Instagram | Instagram |
| Telegram | Telegram |
| Discord | Discord |
| Reddit | Reddit |
| WhatsApp | WhatsApp |
| Line | Line |
| LinkedIn | LinkedIn |
| Pinterest | Pinterest |
| Tumblr | Tumblr |
| Snapchat | Snap |
| Threads | Threads (Meta) |
| Clubhouse | Clubhouse |
| KakaoTalk | KakaoTalk |

---

### 🎮 游戏平台（代理）

| 名称 | 说明 |
|------|------|
| Steam | Steam |
| PlayStation | PlayStation |
| Xbox | Xbox |
| Nintendo | 任天堂 |
| Blizzard | 暴雪 |
| Epic | Epic Games |
| EA | EA Games |
| Riot | Riot Games |
| Rockstar | Rockstar |
| Ubisoft | 育碧 |
| Supercell | Supercell 手游 |
| HoYoverse | 米哈游 |
| HoYoPlay | 米哈游启动器 |
| Hearthstone | 炉石传说 |
| Overwatch | 守望先锋 |
| DiabloIII | 暗黑3 |
| WorldofWarcraft | 魔兽世界 |
| Garena | Garena |

---

### 💻 开发者工具（代理）

| 名称 | 说明 |
|------|------|
| GitHub | GitHub |
| GitLab | GitLab |
| Docker | Docker Hub |
| Jetbrains | JetBrains IDE |
| Stackexchange | Stack Overflow |
| Npmjs | NPM |
| Python | Python/PyPI |
| Vercel | Vercel |
| Heroku | Heroku |
| DigitalOcean | DigitalOcean |
| Atlassian | Jira/Confluence |
| SourceForge | SourceForge |
| Apifox | Apifox |
| HashiCorp | Terraform 等 |

---

### 🍎 苹果生态（直连/代理）

| 名称 | 说明 |
|------|------|
| Apple | Apple 通用 |
| AppStore | App Store |
| AppleMusic | Apple Music |
| AppleTV | Apple TV+ |
| AppleDev | Apple 开发者 |
| AppleMail | iCloud Mail |
| AppleMedia | Apple 媒体服务 |
| iCloud | iCloud |
| Siri | Siri |
| TestFlight | TestFlight |
| FindMy | 查找 |

---

### Ⓜ️ 微软生态（直连/代理）

| 名称 | 说明 |
|------|------|
| Microsoft | Microsoft 通用 |
| MicrosoftEdge | Edge |
| OneDrive | OneDrive |
| Teams | Microsoft Teams |
| Outlook | Outlook（Mail内） |
| Xbox | Xbox |
| LinkedIn | LinkedIn |
| Copilot | Copilot |

---

### 🛒 购物 / 电商（代理）

| 名称 | 说明 |
|------|------|
| Amazon | Amazon |
| eBay | eBay |
| Bestbuy | Best Buy |
| Shopify | Shopify |
| Shopee | Shopee |
| Nike | Nike |
| Adidas | Adidas |

---

### 💰 金融支付（代理）

| 名称 | 说明 |
|------|------|
| PayPal | PayPal |
| Stripe | Stripe |
| Visa | VISA |
| Binance | 币安 |
| Crypto / Cryptocurrency | 加密货币 |

---

### 📰 新闻媒体（代理）

| 名称 | 说明 |
|------|------|
| BBC | BBC |
| CNN | CNN |
| NYTimes | 纽约时报 |
| Bloomberg | 彭博 |
| VOA | 美国之音 |
| RTHK | 香港电台 |
| Reuters | 路透社 |
| Nikkei | 日经 |

---

### 🔒 隐私 / 安全

| 名称 | 说明 |
|------|------|
| EasyPrivacy | 隐私保护 |
| Hijacking | 防劫持 |
| BlockHttpDNS | 阻止 HTTP DNS |
| Advertising | 广告（完整版） |
| AdvertisingLite | 广告（精简版，已集成） |
| AdvertisingMiTV | 小米电视广告 |
| PrivateTracker | PT站 |

---

### 🌐 DNS / 网络基础设施

| 名称 | 说明 |
|------|------|
| DNS | DNS 服务 |
| ChinaDNS | 国内 DNS |
| Cloudflare | Cloudflare |
| Cloudflarecn | Cloudflare 中国 |
| Akamai | Akamai CDN |
| GoogleFCM | Google FCM 推送 |
| NTPService | NTP 时间服务 |

---

## 🎯 推荐扩展配置

### 如果你想更细粒度，可以在默认规则基础上添加：

```yaml
# 在 routing.json 的 rules 数组中追加：

# 1. 已有 AI 服务改为远程规则（更全）
{
  "gfw": true,
  "name": "🤖 AI服务",
  "urls": [
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/OpenAI/OpenAI.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Claude/Claude.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Gemini/Gemini.yaml"
  ]
}

# 2. 单独的 游戏平台 策略组
{
  "gfw": true,
  "name": "🎮 游戏平台",
  "urls": [
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Steam/Steam.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/PlayStation/PlayStation.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Xbox/Xbox.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Nintendo/Nintendo.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Blizzard/Blizzard.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Epic/Epic.yaml"
  ]
}

# 3. 更多流媒体
{
  "gfw": true,
  "name": "🎬 流媒体",
  "urls": [
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/YouTube/YouTube.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Netflix/Netflix.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Disney/Disney.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/HBO/HBO.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Spotify/Spotify.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/TikTok/TikTok.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Twitch/Twitch.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/AmazonPrimeVideo/AmazonPrimeVideo.yaml"
  ]
}

# 4. 更多社交
{
  "gfw": true,
  "name": "🐦 社交平台",
  "urls": [
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Twitter/Twitter.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Facebook/Facebook.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Instagram/Instagram.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Telegram/Telegram.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Discord/Discord.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Reddit/Reddit.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/WhatsApp/WhatsApp.yaml",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/Pinterest/Pinterest.yaml"
  ]
}
```

---

## 📊 规则数量参考

| 规则 | 大约域名数 | 文件大小 |
|------|-----------|---------|
| China | 10000+ | 大 |
| ChinaMax | 20000+ | 很大 |
| Global | 15000+ | 很大 |
| Google | 500+ | 中 |
| YouTube | 100+ | 小 |
| Netflix | 50+ | 小 |
| GitHub | 30+ | 小 |
| Advertising | 50000+ | 很大 |
| AdvertisingLite | 10000+ | 大 |

> 💡 建议：规则越多加载越慢，按需选择。China 和 Global 是大而全的选择，细分类更灵活。
