#!/usr/bin/env bash
set -euo pipefail

cd /workspaces/WineVault/frontend
npm ci
cd ../backend
go mod download

air -c ../.devcontainer/air.toml &
backend_pid=$!
cd ../frontend
npm run dev -- --port 3000 &
frontend_pid=$!

cleanup() {
  kill -TERM "$backend_pid" "$frontend_pid" 2>/dev/null || true
  wait "$backend_pid" "$frontend_pid" 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

# End the container if either watcher exits, so failures stay visible.
wait -n "$backend_pid" "$frontend_pid"
