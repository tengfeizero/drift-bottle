#!/usr/bin/env bash
# 漂流瓶服务管理脚本
# 说明：本脚本随各版本目录一起分发，位于可执行文件同一目录。
# 用法：chmod +x manage.sh && ./manage.sh {start|stop|restart|status|log|help}
#
# 约定：
#   - 运行数据（SQLite 数据库、上传文件）存放在版本目录外的共享目录，升级版本不丢数据。
#   - 日志与 PID 文件保存在本版本目录内。
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION="$(basename "$SCRIPT_DIR")"

LOG_DIR="$SCRIPT_DIR/logs"
PID_FILE="$SCRIPT_DIR/app.pid"
LOG_FILE="$LOG_DIR/service.log"

# 共享数据目录：默认取版本目录的上一级下的 data/，可用 DATA_DIR 覆盖
DATA_DIR="${DATA_DIR:-$(cd "$SCRIPT_DIR/.." && pwd)/data}"
PORT="${PORT:-8080}"

# 定位可执行文件
BIN=""
for name in driftbottle driftbottle_linux driftbottle.exe; do
  if [ -f "$SCRIPT_DIR/$name" ]; then
    BIN="$SCRIPT_DIR/$name"
    break
  fi
done
if [ -z "$BIN" ]; then
  for f in "$SCRIPT_DIR"/*; do
    [ -f "$f" ] || continue
    [ "$(basename "$f")" = "manage.sh" ] && continue
    if [ -x "$f" ]; then
      BIN="$f"
      break
    fi
  done
fi

is_running() {
  [ -f "$PID_FILE" ] || return 1
  local pid
  pid="$(cat "$PID_FILE" 2>/dev/null || true)"
  [ -n "$pid" ] || return 1
  kill -0 "$pid" 2>/dev/null
}

start() {
  if is_running; then
    echo "服务已在运行 (PID $(cat "$PID_FILE"))"
    return 0
  fi
  if [ -z "$BIN" ]; then
    echo "未找到可执行文件" >&2
    exit 1
  fi
  mkdir -p "$LOG_DIR" "$DATA_DIR"
  chmod +x "$BIN" 2>/dev/null || true
  echo "启动 $VERSION ... (端口 $PORT, 数据目录 $DATA_DIR)"
  nohup "$BIN" -port "$PORT" -data "$DATA_DIR" >>"$LOG_FILE" 2>&1 &
  echo $! >"$PID_FILE"
  sleep 1
  if is_running; then
    echo "已启动 (PID $(cat "$PID_FILE"))，日志: $LOG_FILE"
  else
    echo "启动失败，请查看日志: $LOG_FILE" >&2
    rm -f "$PID_FILE"
    exit 1
  fi
}

stop() {
  if ! is_running; then
    echo "服务未运行"
    rm -f "$PID_FILE"
    return 0
  fi
  local pid
  pid="$(cat "$PID_FILE")"
  echo "停止服务 (PID $pid) ..."
  kill "$pid" 2>/dev/null || true
  local i
  for i in $(seq 1 20); do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.5
  done
  if kill -0 "$pid" 2>/dev/null; then
    echo "优雅退出超时，强制结束"
    kill -9 "$pid" 2>/dev/null || true
  fi
  rm -f "$PID_FILE"
  echo "已停止"
}

status() {
  if is_running; then
    echo "状态: 运行中"
    echo "PID: $(cat "$PID_FILE")"
  else
    echo "状态: 已停止"
  fi
  echo "版本: $VERSION"
  echo "端口: $PORT"
  echo "数据目录: $DATA_DIR"
  echo "可执行: ${BIN:-未找到}"
  echo "日志: $LOG_FILE"
}

log() {
  if [ ! -f "$LOG_FILE" ]; then
    echo "暂无日志: $LOG_FILE"
    return 0
  fi
  tail -n "${LOG_LINES:-100}" -f "$LOG_FILE"
}

usage() {
  cat <<'EOF'
用法: ./manage.sh {start|stop|restart|status|log|help}
  start    启动服务
  stop     停止服务
  restart  重启服务
  status   查看服务状态
  log      实时查看日志（可用 LOG_LINES 指定初始行数，默认 100）
  help     显示本帮助

环境变量:
  PORT       监听端口（默认 8080）
  DATA_DIR   数据目录（默认 版本目录上一级/data）
  LOG_LINES  log 命令初始显示行数（默认 100）
EOF
}

case "${1:-}" in
  start) start ;;
  stop) stop ;;
  restart)
    stop
    start
    ;;
  status) status ;;
  log) log ;;
  help | -h | --help | "") usage ;;
  *)
    echo "未知命令: $1" >&2
    usage
    exit 1
    ;;
esac
