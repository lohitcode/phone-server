#!/usr/bin/env bash
# phone-adb: enable wireless ADB on port 5555 for the current session.
#
# WHY THIS EXISTS
#   On a non-rooted phone, ADB-over-network can't survive a reboot — Wireless
#   debugging resets on reboot (Nothing OS) and `adb tcpip` reverts to USB.
#   So after each reboot you re-trigger it once over USB, then go wireless.
#
# USAGE
#   1. Plug the phone into the Mac via USB.
#   2. Run:  ./phone-adb.sh        (or:  bash phone-adb.sh)
#   3. Unplug USB. Wireless ADB on 192.168.0.2:5555 works until next reboot.
#
# RECOVERY
#   If 192.168.0.2:5555 ever stops working, just re-run this script with USB in.

set -euo pipefail
IP=192.168.0.2
PORT=5555

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
