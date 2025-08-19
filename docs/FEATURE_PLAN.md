# ProxyFilter 总体功能规划

> 目标：把 ProxyFilter 做成一个可登录管理、多源订阅聚合、规则过滤、统一分流、可安全分享订阅链接的 Clash/Mihomo 订阅管理服务。

---

## 1. 总体定位

ProxyFilter 不只是简单的 Clash YAML 过滤器，而是一个订阅聚合分发平台：

```text
本地节点 / 远程订阅源
        ↓
定时刷新 + 缓存
        ↓
订阅组 / Profile 聚合
        ↓
正则过滤 / JS 处理 / 字段覆写 / 重命名
        ↓
统一分流规则生成
        ↓
安全 token 订阅链接 /sub/{token}
        ↓
提供给 Clash / Mihomo / 其他用户使用
```

---

## 2. 功能模块规划

## 2.1 登录功能

### 目标

提供基础后台保护，避免任意人访问管理页面、创建订阅、修改规则。

### 功能

| 功能 | 说明 |
|---|---|
| 管理员登录 | 用户名 + 密码 |
| Session/Cookie | 登录后访问管理页面 |
| API 鉴权 | `/api/*` 需要登录 |
| 订阅链接免登录 | `/sub/{token}` 供外部客户端访问，不需要登录 |
| 修改密码 | 后续支持 |

### 配置建议

```json
{
  "auth": {
    "enabled": true,
    "username": "admin",
    "password_hash": "bcrypt_hash",
    "session_secret": "random-secret"
  }
}
```

### 安全要求

- 密码不能明文保存
- 使用 bcrypt 哈希
- Session secret 自动生成或配置
- `/sub/{token}` 使用随机 token，不允许用 name/path 直接访问

---

## 2.2 添加订阅源

订阅源分两类：

1. 远程订阅
2. 本地订阅内容

### 2.2.1 远程 Clash/Mihomo YAML 订阅

示例：

```text
https://example.com/clash.yaml
https://example.com/sub?token=xxx
```

支持：

- Clash YAML
- Mihomo YAML
- Base64 包装内容
- URI 列表内容

### 2.2.2 本地订阅内容

用户可直接粘贴：

```text
ss://...
ssr://...
vless://...
vmess://...
trojan://...
hysteria2://...
```

也可以粘贴多行 URI：

```text
vless://uuid@host:443?...#HK-01
ss://method:pass@host:8388#SG-01
ssr://...
```

后台保存为一个本地 Source。

### Source 数据模型

```json
{
  "id": "source_token_or_uuid",
  "name": "免费源1",
  "type": "remote_yaml | local_uri | local_yaml",
  "url": "https://...",
  "content": "本地内容，可选",
  "enabled": true,
  "refresh_interval_minutes": 60,
  "last_update": "...",
  "status": 200,
  "latency": 123,
  "error": "",
  "node_count": 100
}
```

### 刷新策略

| 场景 | 行为 |
|---|---|
| 自动刷新 | 最短 1 小时 |
| 手动刷新 | 管理员点击立即刷新 |
| 本地订阅 | 无需远程刷新，但解析结果可缓存 |
| 远程订阅 | 成功后缓存原始内容和解析结果 |
| 失败 | 保留上次成功缓存，可标记 stale |

### 刷新限制

- 全局最小间隔：60 分钟
- 手动刷新不受最小间隔限制，但应做按钮防抖
- 单源拉取超时可配置，默认 10 秒

---

## 2.3 订阅组 / Profile 聚合

### 目标

用户创建一个“聚合订阅 Profile”，可选择多个订阅源，合并后通过安全链接分享：

```text
/sub/{token}
```

### Profile 能力

| 功能 | 说明 |
|---|---|
| 选择订阅源 | 可选多个 Source，不选表示全部 |
| 正则包含 | include regex |
| 正则去除 | exclude regex |
| 类型过滤 | ss / ssr / vmess / vless / trojan / hysteria2 / http / socks5 |
| server 过滤 | ip / domain |
| JS 脚本 | transform(node) 自定义过滤/修改 |
| 字段覆写 | tls / sni / server / port / skip-cert-verify 等 |
| 重命名模板 | `{code}_{tag}` |
| 排序 | region / name / type |
| 分享 token | 128-bit 随机 token |
| 启用/停用 | 可关闭某个聚合订阅 |

### Profile 数据模型

```json
{
  "id": "内部ID，可与token相同或分离",
  "token": "128-bit-random-token",
  "name": "HK+JP聚合",
  "enabled": true,
  "sources": ["source_id_1", "source_id_2"],
  "include": "(?i)香港|HK|🇭🇰",
  "exclude": "公告|套餐|到期|未知",
  "type_filter": "vless|vmess|ss",
  "server_filter": "",
  "script": "function transform(node) { return true; }",
  "overrides": {
    "tls": true,
    "sni": "www.microsoft.com"
  },
  "rename_pattern": "{code}_{tag}",
  "sort_by": "region",
  "routing_profile_id": "default-routing"
}
```

### 处理流程

```text
GET /sub/{token}
    ↓
校验 token 是否存在、是否启用
    ↓
读取 Profile
    ↓
读取选中 Source 的缓存解析结果
    ↓
如果缓存不存在或过期：尝试刷新
    ↓
合并 proxies
    ↓
去重 type/server/port
    ↓
校验必要字段
    ↓
执行过滤：include → exclude → type → server
    ↓
执行 JS transform
    ↓
执行字段覆写
    ↓
执行重命名
    ↓
排序
    ↓
套用统一分流规则
    ↓
输出 YAML
```

### 缓存策略

| 缓存 | Key | TTL |
|---|---|---|
| Source 原始内容 | source:{id}:raw | 1h+ |
| Source 解析结果 | source:{id}:parsed | 1h+ |
| Profile 输出 | sub:{token} | 10min |

要求：

- `/sub/{token}` 读取时优先使用缓存
- Source 刷新成功后清理相关 Profile 输出缓存
- 远程源失败时可使用 stale 缓存

---

## 2.4 支持的订阅类型解析

当前和目标支持：

| 类型 | 状态 |
|---|---|
| Clash YAML | 已支持 |
| Mihomo YAML | 已支持 |
| Base64 URI list | 已支持 |
| 普通 URI list | 已支持 |
| VMess URI | 已支持 |
| Shadowsocks / SS URI | 已支持 |
| SSR URI | 已支持 |
| Trojan URI | 已支持 |
| VLESS URI | 已支持 |
| Hysteria2 URI | 已支持 |
| HTTP 节点 | YAML 中保留 |
| SOCKS5 节点 | YAML 中保留 |

---

## 2.5 统一分流管理

### 目标

所有 Profile 可以选择统一的分流配置，生成标准 Mihomo/Clash 配置，而不是完全依赖第一个订阅源的 `proxy-groups` 和 `rules`。

### Routing Profile

```json
{
  "id": "default-routing",
  "name": "默认分流规则",
  "enabled": true,
  "rules": [
    {
      "name": "广告拦截",
      "gfw": false,
      "extraProxies": ["REJECT"],
      "urls": [
        "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/AdvertisingLite/AdvertisingLite_Classical.yaml"
      ]
    },
    {
      "name": "linux.do",
      "gfw": true,
      "payload": [
        "DOMAIN-SUFFIX,linux.do",
        "DOMAIN-SUFFIX,idcflare.com"
      ]
    },
    {
      "name": "YouTube",
      "gfw": true,
      "urls": [
        "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/YouTube/YouTube.yaml",
        "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Clash/YouTubeMusic/YouTubeMusic.yaml"
      ]
    }
  ]
}
```

### Rule Item 字段

| 字段 | 类型 | 说明 |
|---|---|---|
| name | string | 规则名称，也是策略组名称 |
| gfw | bool | true 默认走代理；false 默认 DIRECT 优先 |
| urls | string/string[] | rule-provider 远程规则集 URL |
| payload | string/string[] | 直接内联 Clash rule |
| extraProxies | string/string[] | 额外代理选项，如 REJECT |

### 生成逻辑

#### rule-provider

对于 urls：

```yaml
rule-providers:
  YouTube-rule:
    type: http
    interval: 86400
    behavior: classical
    format: yaml
    url: https://...
```

#### payload

对于 payload：

```text
DOMAIN-SUFFIX,linux.do,linux.do
DOMAIN-SUFFIX,idcflare.com,linux.do
```

#### gfw=true 策略组

```yaml
- name: YouTube
  type: select
  proxies:
    - 自动选择(最低延迟)
    - 负载均衡
    - DIRECT
  include-all: true
```

#### gfw=false 策略组

```yaml
- name: Microsoft
  type: select
  proxies:
    - DIRECT
    - 自动选择(最低延迟)
    - 负载均衡
  include-all: true
```

### 默认生成的 proxy-groups

```yaml
proxy-groups:
  - name: 国内网站
    type: select
    proxies: [DIRECT, 自动选择(最低延迟), 负载均衡]
    include-all: true

  # gfw=false 规则组...

  - name: 国外网站
    type: select
    proxies: [DIRECT, 自动选择(最低延迟), 负载均衡]
    include-all: true

  # gfw=true 规则组...

  - name: 被墙网站
    type: select
    proxies: [自动选择(最低延迟), 负载均衡, DIRECT]
    include-all: true

  - name: 自动选择(最低延迟)
    type: url-test
    tolerance: 20
    include-all: true
    url: https://play-lh.googleusercontent.com/...

  - name: 负载均衡
    type: load-balance
    include-all: true
    hidden: true
    strategy: sticky-sessions
    url: https://play-lh.googleusercontent.com/...
```

### 默认 rules

```yaml
rules:
  - IP-CIDR,127.0.0.0/8,DIRECT,no-resolve
  - IP-CIDR6,::1/128,DIRECT,no-resolve
  # payload 和 RULE-SET...
  - GEOSITE,gfw,被墙网站
  - GEOIP,CN,国内网站
  - MATCH,国外网站
```

### DNS / geox-url 模板

统一分流配置可选择是否覆写输出配置中的：

- mode
- dns
- geox-url
- proxy-groups
- rule-providers
- rules

默认建议统一生成，避免多个订阅源的规则冲突。

---

## 3. 前端页面规划

### 3.1 登录页

- 用户名
- 密码
- 记住登录

### 3.2 订阅源管理

已有基础，需增强：

- Source 名称
- Source 类型：远程 YAML / 本地 URI / 本地 YAML
- 本地内容编辑框
- 启用/停用
- 刷新间隔配置，最短 1 小时
- 手动刷新
- 查看节点数量
- 查看最近错误

### 3.3 聚合订阅 Profile 管理

已有基础，需增强：

- token 显示/复制
- token 重置
- 启用/停用
- 访问统计
- 输出格式选择
- 指定统一分流规则
- 预览过滤结果

### 3.4 统一分流管理

新页面：

- Routing Profile 列表
- Rule Item 增删改
- urls 编辑器
- payload 编辑器
- extraProxies 编辑器
- gfw 开关
- 预览生成的 proxy-groups/rules

---

## 4. 后端模块规划

```text
internal/auth          登录、Session、密码哈希
internal/source        Source 模型、远程/本地订阅管理
internal/rule          Profile 过滤规则引擎
internal/routing       统一分流规则生成器
internal/storage       config.json 或 SQLite 持久化
internal/handler       REST API
internal/provider      远程拉取和解析
internal/parser        YAML/Base64/URI 解析
internal/cache         缓存
internal/scheduler     定时刷新
```

---

## 5. 推荐开发顺序

### Phase 1：安全与基础持久化

1. 登录功能
2. Source 模型重构，支持本地订阅
3. Profile token 独立化：id 和 token 分离
4. Profile 启用/停用、重置 token

### Phase 2：订阅聚合完善

1. Source 缓存解析结果
2. `/sub/{token}` 读取缓存聚合
3. 手动刷新 Source
4. 最短 1 小时自动刷新限制
5. stale 缓存兜底

### Phase 3：统一分流管理

1. Routing Profile 模型
2. rule-provider 生成
3. payload rule 生成
4. proxy-groups 模板生成
5. Profile 选择 Routing Profile

### Phase 4：前端完善

1. 登录页
2. Source 增强编辑器
3. Profile 高级编辑器
4. Routing Profile 管理页面
5. 预览/测试面板

---

## 6. 当前已完成能力

| 功能 | 状态 |
|---|---|
| 多源拉取 | 已完成 |
| Clash YAML 解析 | 已完成 |
| URI 解析 | 已完成 |
| 正则过滤 | 已完成 |
| JS transform | 已完成 |
| 字段覆写 | 已完成 |
| 短码重命名 | 已完成 |
| `/sub/{token}` 输出 | 已完成 |
| Profile 前端基础管理 | 已完成 |
| Profile 持久化 | 已完成 |
| 128-bit token | 已完成 |

---

## 7. 当前待做重点

| 优先级 | 任务 |
|---|---|
| P0 | 登录功能 |
| P0 | Source 重构，支持本地订阅内容 |
| P0 | Profile id/token 分离、token 重置、启停 |
| P1 | Source 解析结果缓存，`/sub` 只读缓存优先 |
| P1 | 统一分流 Routing Profile |
| P1 | 前端 Routing 管理页 |
| P2 | 访问日志/订阅统计 |
| P2 | 输出格式选择 |
| P2 | SQLite 替代 config.json |
