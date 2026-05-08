#!/usr/bin/env bash
# setup-tma.sh — Telegram Mini App local development setup
#
# Usage:
#   ./scripts/setup-tma.sh [--ngrok-token TOKEN]
#
# What this script does:
#   1. Validates required env vars
#   2. Starts an ngrok HTTPS tunnel to port 8080
#   3. Prints the BotFather commands you need to run
#   4. Optionally patches .env with the tunnel URL

set -euo pipefail

# ── Colours ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

info()  { echo -e "${CYAN}[info]${RESET}  $*"; }
ok()    { echo -e "${GREEN}[ok]${RESET}    $*"; }
warn()  { echo -e "${YELLOW}[warn]${RESET}  $*"; }
error() { echo -e "${RED}[error]${RESET} $*"; exit 1; }

# ── Parse args ───────────────────────────────────────────────────────────────
NGROK_TOKEN=""
while [[ $# -gt 0 ]]; do
  case $1 in
    --ngrok-token) NGROK_TOKEN="$2"; shift 2 ;;
    *) error "Unknown argument: $1" ;;
  esac
done

# ── Check prerequisites ───────────────────────────────────────────────────────
info "Checking prerequisites..."

command -v ngrok  >/dev/null 2>&1 || error "ngrok not found. Install from https://ngrok.com/download"
command -v curl   >/dev/null 2>&1 || error "curl not found"
command -v jq     >/dev/null 2>&1 || { warn "jq not found — URL extraction will be limited"; JQ_MISSING=1; }

# ── Load env ─────────────────────────────────────────────────────────────────
ENV_FILE="${ENV_FILE:-.env}"
if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC2046
  export $(grep -v '^#' "$ENV_FILE" | grep -v '^$' | xargs)
  info "Loaded $ENV_FILE"
else
  warn ".env not found — copy .env.example and fill in TELEGRAM_BOT_TOKEN"
fi

[[ -n "${TELEGRAM_BOT_TOKEN:-}" ]] || error "TELEGRAM_BOT_TOKEN is not set. Add it to .env"

BOT_TOKEN="$TELEGRAM_BOT_TOKEN"

# ── Authenticate ngrok (if token provided) ────────────────────────────────────
if [[ -n "$NGROK_TOKEN" ]]; then
  info "Saving ngrok auth token..."
  ngrok config add-authtoken "$NGROK_TOKEN"
fi

# ── Start ngrok tunnel ────────────────────────────────────────────────────────
info "Starting ngrok tunnel on port 8080..."

# Kill any existing ngrok instance
pkill -f "ngrok http" 2>/dev/null || true
sleep 1

# Start ngrok in background
ngrok http 8080 --log=stdout > /tmp/ngrok.log 2>&1 &
NGROK_PID=$!
info "ngrok PID: $NGROK_PID"

# Wait for tunnel to be ready
TUNNEL_URL=""
for i in {1..15}; do
  sleep 1
  if [[ "${JQ_MISSING:-0}" == "1" ]]; then
    TUNNEL_URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | grep -o '"public_url":"https://[^"]*"' | head -1 | cut -d'"' -f4)
  else
    TUNNEL_URL=$(curl -s http://localhost:4040/api/tunnels 2>/dev/null | jq -r '.tunnels[] | select(.proto=="https") | .public_url' 2>/dev/null | head -1)
  fi
  [[ -n "$TUNNEL_URL" ]] && break
  echo -n "."
done
echo

[[ -n "$TUNNEL_URL" ]] || error "ngrok tunnel did not start. Check /tmp/ngrok.log"
ok "Tunnel: $TUNNEL_URL"

# ── Patch .env with tunnel URL ────────────────────────────────────────────────
if [[ -f "$ENV_FILE" ]]; then
  if grep -q "^TELEGRAM_APP_URL=" "$ENV_FILE"; then
    sed -i.bak "s|^TELEGRAM_APP_URL=.*|TELEGRAM_APP_URL=$TUNNEL_URL|" "$ENV_FILE"
  else
    echo "TELEGRAM_APP_URL=$TUNNEL_URL" >> "$ENV_FILE"
  fi
  ok "Updated TELEGRAM_APP_URL in $ENV_FILE"
fi

# ── Print BotFather instructions ───────────────────────────────────────────────
echo
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo -e "${BOLD}  BotFather Setup — run these commands in Telegram${RESET}"
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo
echo -e "${CYAN}1. Open @BotFather in Telegram${RESET}"
echo
echo -e "${CYAN}2. Enable Mini App:${RESET}"
echo -e "   /newapp"
echo -e "   → select your bot"
echo -e "   → Title: Lava Crash"
echo -e "   → Description: Crash game powered by Lava Platform"
echo -e "   → Photo: upload any 640×360 image (or skip)"
echo -e "   → Type: Web App"
echo -e "   → URL: ${YELLOW}${TUNNEL_URL}${RESET}"
echo
echo -e "${CYAN}3. Set the Menu Button (shows Web App button in chat):${RESET}"
echo -e "   /setmenubutton"
echo -e "   → select your bot"
echo -e "   → Button text: Play Now"
echo -e "   → URL: ${YELLOW}${TUNNEL_URL}${RESET}"
echo
echo -e "${CYAN}4. Set bot commands (optional):${RESET}"
echo -e "   /setcommands"
echo -e "   → select your bot"
echo -e "   → Paste:"
echo -e "     play - Launch the game"
echo -e "     help - How to play"
echo
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo
echo -e "${CYAN}5. Launch sequence (separate terminals):${RESET}"
echo
echo -e "   ${BOLD}Terminal 1 — Backend:${RESET}"
echo -e "   docker compose up -d postgres redis"
echo -e "   go run ./cmd/api"
echo
echo -e "   ${BOLD}Terminal 2 — Frontend:${RESET}"
echo -e "   cd frontend && npm run dev"
echo
echo -e "   ${BOLD}Or serve frontend via backend (production mode):${RESET}"
echo -e "   cd frontend && npm run build"
echo -e "   # Backend serves dist/ at /"
echo
echo -e "${CYAN}6. Test auth endpoint:${RESET}"
echo -e "   curl -s ${TUNNEL_URL}/tma/health | jq"
echo
echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${RESET}"
echo
ok "ngrok is running (PID $NGROK_PID). Press Ctrl+C to stop."
echo -e "   Dashboard: ${CYAN}http://localhost:4040${RESET}"
echo

# ── Keep alive ────────────────────────────────────────────────────────────────
trap "kill $NGROK_PID 2>/dev/null; info 'ngrok stopped.'" INT TERM
wait $NGROK_PID
