#!/usr/bin/env bash

set -euo pipefail

HOST="${PHONE_HOST:-phone}"
ENV_FILE=".env"
REMOTE_CONFIG_DIR=".config/cloudflared"
REMOTE_TOKEN_FILE="$REMOTE_CONFIG_DIR/tunnel-token"

if [[ -s "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE"
  set +a
fi

if [[ -z "${CLOUDFLARE_TUNNEL_TOKEN:-}" ]]; then
  read -r -s -p "Paste the NEW Cloudflare tunnel token: " token
  printf '\n'

  if [[ -z "$token" ]]; then
    echo "The tunnel token cannot be empty." >&2
    exit 1
  fi

  printf 'CLOUDFLARE_TUNNEL_TOKEN=%s\n' "$token" > "$ENV_FILE"
  CLOUDFLARE_TUNNEL_TOKEN="$token"
  unset token
  chmod 600 "$ENV_FILE"
fi

TOKEN_UPLOAD="$(mktemp -t cloudflare-tunnel-token.XXXXXX)"
cleanup() {
  rm -f "$TOKEN_UPLOAD"
}
trap cleanup EXIT
printf '%s\n' "$CLOUDFLARE_TUNNEL_TOKEN" > "$TOKEN_UPLOAD"
chmod 600 "$TOKEN_UPLOAD"

echo "Creating secure Cloudflare directories on $HOST..."
ssh "$HOST" "mkdir -p \"\$HOME/$REMOTE_CONFIG_DIR\" \"\$HOME/.termux/boot\""

echo "Uploading the tunnel token..."
scp "$TOKEN_UPLOAD" "$HOST:$REMOTE_TOKEN_FILE.new"

echo "Installing the Termux:Boot launcher..."
ssh "$HOST" 'sh -s' <<'REMOTE_SCRIPT'
set -eu

CONFIG_DIR="$HOME/.config/cloudflared"
TOKEN_FILE="$CONFIG_DIR/tunnel-token"
LOG_FILE="$CONFIG_DIR/cloudflared.log"
BOOT_SCRIPT="$HOME/.termux/boot/start-cloudflared"

mv "$CONFIG_DIR/tunnel-token.new" "$TOKEN_FILE"
chmod 600 "$TOKEN_FILE"

cat > "$BOOT_SCRIPT" <<'BOOT_SCRIPT_CONTENT'
#!/data/data/com.termux/files/usr/bin/sh

termux-wake-lock

TOKEN_FILE="$HOME/.config/cloudflared/tunnel-token"
LOG_FILE="$HOME/.config/cloudflared/cloudflared.log"

if [ -s "$TOKEN_FILE" ] && ! pgrep -f '[c]loudflared tunnel run' >/dev/null; then
  nohup cloudflared tunnel run --token-file "$TOKEN_FILE" \
    >> "$LOG_FILE" 2>&1 < /dev/null &
fi
BOOT_SCRIPT_CONTENT

chmod 700 "$BOOT_SCRIPT"
"$BOOT_SCRIPT"
sleep 5

if pgrep -f '[c]loudflared tunnel run' >/dev/null; then
  echo "Cloudflare Tunnel is running."
  pgrep -af '[c]loudflared tunnel run'
else
  echo "Cloudflare Tunnel did not start. Recent log output:" >&2
  tail -n 30 "$LOG_FILE" >&2 || true
  exit 1
fi
REMOTE_SCRIPT

echo
echo "Setup complete. Configure the Cloudflare public hostname service as:"
echo "  Type: HTTP"
echo "  URL:  http://127.0.0.1:8080"
echo
echo "Phone logs: ssh $HOST 'tail -f ~/.config/cloudflared/cloudflared.log'"
