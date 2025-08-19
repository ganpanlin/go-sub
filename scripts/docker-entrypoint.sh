#!/bin/sh
set -eu

DATA_DIR="${DATA_DIR:-/app/data}"
DEFAULT_DATA="/app/default-data"
PORT="${PORT:-8080}"

mkdir -p "$DATA_DIR/cache"

# 首次启动：如果数据目录为空，从镜像内复制默认数据
if [ ! -f "$DATA_DIR/sources.json" ]; then
  echo "[init] First run, copying default data..."
  cp "$DEFAULT_DATA/sources.json"   "$DATA_DIR/sources.json"
  cp "$DEFAULT_DATA/profiles.json"  "$DATA_DIR/profiles.json"
  cp "$DEFAULT_DATA/routing.json"   "$DATA_DIR/routing.json"
  [ -f "$DEFAULT_DATA/rulesets.json" ] && cp "$DEFAULT_DATA/rulesets.json" "$DATA_DIR/rulesets.json"
  echo "[init] Done. Edit data/ to customize."
fi

exec /app/proxy-filter \
  -port "$PORT" \
  -data-dir "$DATA_DIR" \
  -frontend-dir "${FRONTEND_DIR:-/app/frontend}" \
  -http-timeout "${HTTP_TIMEOUT:-10}" \
  -cache-ttl "${CACHE_TTL:-60}" \
  -filter-cache-ttl "${FILTER_CACHE_TTL:-10}"
