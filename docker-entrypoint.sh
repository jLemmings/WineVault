#!/bin/sh
set -eu

export DATABASE_URL="${DATABASE_URL:-postgres://winevault:winevault_dev@postgres:5432/winevault?sslmode=disable}"
export HTTP_ADDR="${HTTP_ADDR:-:8080}"
export AUTH_COOKIE_SECURE="${AUTH_COOKIE_SECURE:-false}"
export PORT="${PORT:-3000}"

cd /app/backend
/usr/local/bin/winevault &
app_pid=$!

trap 'kill "$app_pid" 2>/dev/null || true' EXIT INT TERM

cd /app/frontend
exec node .output/server/index.mjs
