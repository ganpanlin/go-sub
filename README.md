# ProxyFilter

代理订阅聚合管理系统。多个远程订阅源 → 合并去重 → 过滤重命名 → 生成 Clash/Mihomo/Surge/Loon/sing-box/QuantumultX 等客户端配置。

## 功能

- **订阅源管理**：远程 URL、本地 URI、本地 YAML，支持测试/刷新/批量操作
- **聚合订阅**：正则过滤、JS 脚本、字段覆写、节点重命名、按区域排序
- **分流规则**：内置规则库（12 分类 58 条）一键勾选，支持自定义规则集和内联规则
- **多格式输出**：Clash、Surge、Loon、Quantumult X、sing-box、Base64 URI
- **节点健康检测**：TCP 连通性测试，显示每个节点延迟
- **访问统计**：订阅链接访问次数和最近访问记录
- **配置导入导出**：一键备份恢复所有配置
- **管理员登录**：JWT 认证，首次启动引导设置密码

## 快速开始

### go run（开发）

```bash
git clone https://github.com/ganpanlin/go-sub.git
cd go-sub
go run ./cmd/proxy-filter
```

打开 `http://localhost:8080`，首次需设置管理员密码。

### Docker Compose（推荐）

```bash
git clone https://github.com/ganpanlin/go-sub.git
cd go-sub
docker compose up -d --build
```

首次启动自动初始化默认数据（20 个订阅源、1 个 profile、默认分流规则）。

### Docker 单独运行

```bash
docker build -t proxy-filter:local .
docker run -d -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/frontend:/app/frontend \
  proxy-filter:local
```

## 分流规则

### 规则库

管理台内置规则库，支持分类勾选：

- 🤖 AI 服务（OpenAI、Claude、Gemini、Copilot、DeepSeek）
- 🎬 流媒体（YouTube、Netflix、Disney+、HBO、Spotify、TikTok...）
- 🐦 社交平台（Twitter、Facebook、Instagram、Telegram、Discord...）
- 🎮 游戏平台（Steam、PlayStation、Xbox、Nintendo、暴雪、Epic...）
- 💻 开发工具（GitHub、GitLab、Docker、JetBrains...）
- 💱 交易所（Binance、OKX、Bybit、Gate.io、Bitget...）
- 🍎 苹果服务、Ⓜ️ 微软服务、📢 谷歌服务
- 🔒 隐私安全、🌐 基础设施、🔧 常用工具、🇨🇳 国内直连

### 自定义规则集

支持创建自定义规则集，对外提供 YAML 格式的 rule-provider 端点：

```
GET /rules/{id}.yaml
```

可在分流规则中直接引用自建规则集 URL。

## 输出格式

通过订阅链接的 `?type=` 参数指定：

| 参数值 | 格式 |
|--------|------|
| `clash`（默认） | Clash/Mihomo YAML |
| `surge` | Surge |
| `loon` | Loon |
| `quantumult` / `quanx` | Quantumult X |
| `singbox` / `sing-box` | sing-box JSON |
| `base64` / `uri` | Base64 编码 URI 列表 |

## 支持的节点协议

VMess、Shadowsocks、ShadowsocksR、Trojan、VLESS、Hysteria2

## 启动参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-port` | `8080` | 服务监听端口 |
| `-config` | `config.json` | 配置文件路径 |
| `-data-dir` | `data` | 数据目录 |
| `-frontend-dir` | `frontend` | 前端目录 |
| `-http-timeout` | `10` | 远程请求超时（秒） |
| `-cache-ttl` | `60` | 源缓存时间（分钟） |
| `-filter-cache-ttl` | `10` | 输出缓存时间（分钟） |

## 安全建议

- 首次部署后立即创建管理员密码
- 不要把 `data/` 目录提交到公开仓库
- 暴露到公网时建议放在反向代理后面并启用 HTTPS
- 订阅源 URL 可能包含 token，分享日志前注意脱敏

## 许可证

MIT
