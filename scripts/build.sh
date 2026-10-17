#!/usr/bin/env bash
set -euo pipefail

# 切到仓库根目录（脚本位于 scripts/ 下）
cd "$(dirname "$0")/.."

echo "============================================"
echo " DriftBottle - Build for Linux (amd64)"
echo "============================================"

echo "[1/5] Build frontend ..."
pushd frontend >/dev/null
npm install
npm run build
popd >/dev/null

echo "[2/5] Copy frontend dist to embed dir ..."
rm -rf backend/web/dist
mkdir -p backend/web/dist
cp -R frontend/dist/. backend/web/dist/

VER="$(date +%y%m%d_%H%M%S)"
VERDIR="dist/$VER"

echo "[3/5] Compile backend (GOOS=linux GOARCH=amd64) ..."
mkdir -p "$VERDIR"
pushd backend >/dev/null
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "../$VERDIR/driftbottle" .
popd >/dev/null

echo "[4/5] Bundle manage.sh ..."
cp scripts/manage.sh "$VERDIR/manage.sh"

echo "[5/5] Done. Output: $VERDIR"
echo "  - driftbottle"
echo "  - manage.sh"
