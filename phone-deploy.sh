#!/usr/bin/env bash

set -euo pipefail

HOST="${PHONE_HOST:-phone}"
APP_NAME="phone-server"
REMOTE_DIR="${PHONE_APP_DIR:-apps/$APP_NAME}"
LOCAL_BINARY="$(mktemp -t "${APP_NAME}.XXXXXX")"

cleanup() {
  rm -f "$LOCAL_BINARY"
}
trap cleanup EXIT

echo "Building $APP_NAME for Android ARM64..."
CGO_ENABLED=0 GOOS=android GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o "$LOCAL_BINARY" .

echo "Uploading to $HOST:$REMOTE_DIR..."
ssh "$HOST" "mkdir -p \"\$HOME/$REMOTE_DIR\""
scp "$LOCAL_BINARY" "$HOST:$REMOTE_DIR/$APP_NAME.new"

echo "Restarting $APP_NAME..."
ssh "$HOST" "\
  set -eu; \
  cd \"\$HOME/$REMOTE_DIR\"; \
  if [ -f '$APP_NAME.pid' ] && kill -0 \$(cat '$APP_NAME.pid') 2>/dev/null; then \
    kill \$(cat '$APP_NAME.pid'); \
    sleep 1; \
  fi; \
  mv '$APP_NAME.new' '$APP_NAME'; \
  chmod 700 '$APP_NAME'; \
  nohup ./'$APP_NAME' >> '$APP_NAME.log' 2>&1 < /dev/null & \
  echo \$! > '$APP_NAME.pid'; \
  sleep 2; \
  curl --fail --silent --show-error http://127.0.0.1:8080/health; \
  printf '\nDeployed successfully. PID: '; \
  cat '$APP_NAME.pid'"
