# ProxyFilter (go-sub) — 后续开发计划

> 基于 2026-06-04 项目审查，按优先级排序的工作计划。

---

## 🔴 紧急修复（1-2 天）

### 1. ~~修复 CORS 中间件方法限制~~ ✅
**文件**: `internal/middleware/cors.go`  
**已完成**: 允许 GET, POST, DELETE, OPTIONS，添加 Max-Age 预检缓存

### 2. ~~补全 VLESS 协议解析~~ ✅
**文件**: `internal/parser/uri.go`  
**已完成**: 支持 VLESS URI 全量解析（ws/grpc/tls/reality/flow），格式：
```
vless://uuid@host:port?type=ws&security=reality&sni=xxx&pbk=xxx...#name
```
需支持 Reality/Xtls 参数。

### 3. ~~补全 Hysteria2 协议解析~~ ✅
**文件**: `internal/parser/uri.go`  
**已完成**: 支持 Hysteria2 URI 解析（sni/带宽/obfs），格式：
```
hysteria2://password@host:port?sni=xxx&insecure=1#name
```

---

## 🟡 短期优化（1-2 周）

### 4. ~~前端自动轮询~~ ✅
**文件**: `frontend/index.html`  
**已完成**: 每 30 秒自动刷新源状态，组件销毁时清理定时器

### 5. ~~添加健康检查端点~~ ✅
**文件**: `internal/handler/health.go`（新建）+ `internal/router/router.go`  
**已完成**: `GET /api/health` → 返回 uptime

### 6. ~~清理无用代码~~ ✅
**文件**: `internal/config/config.go`  
**已完成**: 删除未使用的文件

### 7. ~~HTTP 超时可配置化~~ ✅
**文件**: `internal/provider/provider.go`, `internal/appconfig/config.go`, `cmd/proxy-filter/main.go`  
**已完成**: 新增 `--http-timeout` 命令行参数，默认 10s

### 8. ~~过滤结果缓存失效机制~~ ✅
**文件**: `internal/cache/cache.go`, `internal/provider/provider.go`  
**已完成**: 源刷新成功后自动清除所有 `filter:` 前缀缓存

---

## 🟢 中期增强（2-4 周）

### 9. ~~SSR 协议支持~~ ✅
**文件**: `internal/parser/uri.go`  
**已完成**: SSR URI 完整解析（protocol/method/obfs/params）

### 10. ~~单元测试覆盖~~ ✅
**新增文件**: `cache_test.go`, `geoip_test.go`, `parser_test.go`, `proxy_test.go`, `region_test.go`  
**共 27 个测试用例全部通过**

### 11. ~~扩展区域映射~~ ✅
**文件**: `internal/proxy/region.go`  
**已完成**: 39 个区域，含城市名（东京/大阪/洛杉矶/纽约/莫斯科等）

### 12. ~~IP GeoIP 区域检测~~ ✅ ✨新增
**新增文件**: `internal/geoip/geoip.go`, `internal/handler/geoip.go`  
**功能**: ip-api.com 查询，24h 缓存，域名 DNS 解析，批量并发查询  
**API**: `GET /api/geoip?host=1.2.3.4`

### 13. ~~节点重命名改用短代码~~ ✅ ✨新增
**格式**: `HK_IPLC 01`, `JP_Node A`, `DE_Server`  
**规则**: 优先名称匹配 → 降级 IP GeoIP → 未知用 UN  
**名称清洗**: 自动去除地区关键词（香港/HK/hongkong/东京等）

### 14. REST API 文档

新建 `docs/API.md`，使用 Swagger/OpenAPI 或 Markdown 格式

---

## 🔵 长期规划（1-3 个月）

### 13. 订阅源分组管理

- 支持将多个源归入同一分组（如 "免费节点"、"付费节点"）
- 前端分组筛选视图
- 分组级别过滤

### 14. WebSocket 实时状态推送

- 建立 `/ws` 端点
- 源状态变更时主动推送给前端
- 替代轮询方案

### 15. Docker 化部署

```
# Dockerfile
FROM golang:1.22-alpine AS builder
...
FROM alpine:3.19
COPY proxy-filter /usr/local/bin/
COPY frontend/ /opt/proxy-filter/frontend/
```

### 16. Prometheus 监控

- 端点 `GET /metrics`
- 指标：请求量、延迟、缓存命中率、源可用性

### 17. 过滤结果 Clash 订阅链接生成

- 为过滤结果生成唯一短链接
- 可直接填入 Clash 客户端作为订阅 URL
- 支持 token 认证

### 18. 数据库持久化

- 当前使用内存缓存 + config.json
- 迁移到 SQLite/BoltDB 持久化源列表和配置
- 保留配置导入/导出功能

---

## 里程碑时间线

```
Week 1-2   │ 🔴 修复 CORS + VLESS + Hysteria2
           │ 🟡 前端轮询 + 健康检查 + 清理代码
Week 3-4   │ 🟡 HTTP 超时配置 + 缓存失效 + SSR 解析
Week 5-6   │ 🟢 单元测试 + 区域扩展
Week 7-8   │ 🟢 API 文档 + Docker
Week 9-12  │ 🔵 WebSocket + 订阅源分组
Week 13-16 │ 🔵 Prometheus + 短链接 + 数据库
```

---

## 各任务预估工作量

| 任务 | 工时 |
|------|:--:|
| CORS 修复 | 0.1d |
| VLESS 解析 | 0.5d |
| Hysteria2 解析 | 0.5d |
| 前端自动轮询 | 0.2d |
| 健康检查端点 | 0.2d |
| 清理无用代码 | 0.1d |
| HTTP 超时配置 | 0.2d |
| 缓存失效机制 | 0.5d |
| SSR 协议支持 | 0.5d |
| 单元测试 | 2d |
| 区域扩展 | 0.2d |
| API 文档 | 0.5d |
| 订阅源分组 | 2d |
| WebSocket 推送 | 2d |
| Docker 化 | 0.5d |
| Prometheus | 1d |
| 短链接生成 | 1d |
| 数据库持久化 | 2d |
| **合计** | **~14 天** |