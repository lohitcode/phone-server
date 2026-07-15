#!/usr/bin/env bash
# phone-adb: enable wireless ADB on port 5555 for the current session.
#
# WHY THIS EXISTS
#   On a non-rooted phone, ADB-over-network can't survive a reboot — Wireless
#   debugging resets on reboot (Nothing OS) and `adb tcpip` reverts to USB.
#   So after each reboot you re-trigger it once over USB, then go wireless.
#
# USAGE
#   1. Plug the phone into the development computer via USB.
#   2. Run:  PHONE_IP=192.0.2.10 ./phone-adb.sh
#   3. Unplug USB. Wireless ADB on PHONE_IP:5555 works until next reboot.
#
# RECOVERY
#   If wireless ADB stops working, re-run this script with USB connected.

set -euo pipefail
IP="${PHONE_IP:-${1:-}}"
PORT=5555

if [[ -z "$IP" ]]; then
  echo "Usage: PHONE_IP=<phone-lan-ip> ./phone-adb.sh" >&2
  echo "   or: ./phone-adb.sh <phone-lan-ip>" >&2
  exit 2
fi

echo ">> checking for a USB-attached device..."
if ! adb devices | grep -Eq $'\tdevice$'; then
  echo "   No USB device found. Plug the phone in via USB first, then re-run."
  exit 1
fi

echo ">> telling adbd to listen on tcp:${PORT} ..."
adb tcpip "$PORT"
sleep 1

echo ">> connecting wirelessly to ${IP}:${PORT} ..."
adb connect "${IP}:${PORT}"

echo
echo ">> current devices:"
adb devices
echo
echo "✅ Done. You can unplug USB now. Wireless ADB is live until the next reboot."
