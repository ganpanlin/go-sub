# ProxyFilter

代理订阅聚合管理系统。多个远程订阅源 → 合并去重 → 过滤重命名 → 生成 Clash/Mihomo/Surge/Loon/sing-box/QuantumultX 等客户端配置。

## 功能

- **订阅源管理**：远程 URL、本地 URI、本地 YAML，支持测试/刷新/批量操作
- **聚合订阅**：正则过滤、JS 脚本、字段覆写、节点重命名、按区域排序
- **高延迟过滤**：基于健康检测结果自动剔除离线/高延迟节点（`max_latency`、`drop_dead`），支持按延迟排序
- **分流规则**：内置规则库（12 分类 58 条）一键勾选，支持自定义规则集、内联规则，以及极简/均衡/完整三种**分流模板**一键创建
- **多格式输出**：Clash、Surge、Loon、Quantumult X、sing-box、Base64 URI
- **节点健康检测**：双模式 — TCP 快速连通性检测 + **真实延迟测速**（内嵌 mihomo 内核，通过节点代理请求测速 URL，与客户端 url-test 一致）；结果持久化（`data/health-status.json`）并支持一键应用到订阅
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

> **离线构建**：Go 依赖（含内嵌的 mihomo 真实延迟测速内核）已通过 `go mod vendor` 固化在项目 `vendor/` 目录中，Docker 构建阶段使用 `-mod=vendor` 编译，**不需要访问 Go 模块代理**，克隆仓库后可直接离线打包。依赖变更时执行 `go mod vendor` 更新即可。

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

### 分流模板

在「分流规则」页点击 **从模板创建**，一键生成完整可用的分流配置（基于内置规则库，创建后可再编辑）：

| 模板 | 规则数 | 适用场景 |
|------|--------|----------|
| 极简模式 | 1 | 仅广告拦截，其余走兜底分组，追求客户端性能 |
| 均衡模式 | 8 | AI/流媒体/社交/谷歌走代理，微软/苹果/开发工具直连 |
| 完整模式 | 12 | 全部分类各一个策略组，分流最细 |

对应 API：`GET /api/routing/templates`、`POST /api/routing/apply-template`。

### 高延迟过滤

1. 在聚合订阅列表点击 **健康检测**，选择检测方式（异步任务 + 实时轮询，结果逐个点亮表格，无需等全部完成，可中途取消）：
   - **TCP 快速检测**：几秒内批量测到节点入口的握手延迟，适合筛离线节点
   - **真实延迟测速 (url-test)**：内嵌 mihomo 内核，**真正通过节点代理**请求 `http://www.gstatic.com/generate_204`，测出的延迟与 Clash 客户端显示一致，能识别「TCP 能通但实际不可用/很慢」的节点
2. 检测对话框内设置延迟阈值（如 300ms），点击 **应用到订阅**
3. 订阅输出时自动剔除离线节点和超过阈值的节点；也可在编辑对话框手动配置 `max_latency` / `drop_dead`，或选择 **按延迟** 排序（最快在前，未检测节点次之，离线最后）

两种检测的结果互相覆盖（以最近一次为准）。未检测过的节点始终保留（保守策略），避免换机/清数据后订阅变空。健康检测会忽略订阅自身的延迟过滤配置，保证被过滤的节点也能复测。

**定时自动检测**：启动后立即执行一次「拉取订阅 → 延迟测试 → 预热订阅缓存」链路，之后默认**每 60 分钟**自动重复（检测的每个节点延迟值（ms）与时间戳都持久化到 `data/health-status.json`，阈值 `max_latency` 保存在订阅配置中）：

```json
{
  "health_check_interval_minutes": 60,
  "health_check_mode": "url",
  "health_check_timeout_sec": 5
}
```

- `health_check_interval_minutes`：链路周期（分钟）。**缺省 = 60**；`0` = 关闭
- `health_check_mode`：**`url`（默认，真实延迟测速）**或 `tcp`（快速连通性）
- `health_check_timeout_sec`：单节点超时，默认 tcp 3 秒 / url 5 秒

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

## 配置文件（config.json）

| 字段 | 默认 | 说明 |
|------|------|------|
| `refresh_interval_minutes` | 0（关） | 定时刷新全部订阅源的间隔（分钟） |
| `health_check_interval_minutes` | 0（关） | 定时健康检测的间隔（分钟），保持延迟过滤数据新鲜 |
| `health_check_mode` | tcp | 定时检测模式：`tcp` / `url` |
| `health_check_timeout_sec` | 3/5 | 定时检测单节点超时（秒） |
| `request_ua` | clash | 拉取订阅源时使用的 User-Agent |

启动参数之外，`config.json` 缺失时会从 `config.example.json` 自动生成。定时任务在启动时读取，修改后需重启生效。

## 安全建议

- 首次部署后立即创建管理员密码
- 不要把 `data/` 目录提交到公开仓库
- 暴露到公网时建议放在反向代理后面并启用 HTTPS
- 订阅源 URL 可能包含 token，分享日志前注意脱敏

## 许可证

GPL-3.0。本项目内嵌了 [mihomo](https://github.com/MetaCubeX/mihomo)（GPL-3.0）作为真实延迟测速内核，因此整体许可证需遵循 GPL-3.0。
