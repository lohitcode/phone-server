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
#   2. Run:  ./phone-adb.sh
#   3. Unplug USB. Wireless ADB on 192.168.0.2:5555 works until next reboot.
#
# NETWORK
#   This project reserves 192.168.0.2 for the phone in the router's DHCP
#   settings, so the SSH and ADB configuration remains stable after reboots.
#   Other users can override it with PHONE_IP or the first argument.
#
# RECOVERY
#   If wireless ADB stops working, re-run this script with USB connected.

set -euo pipefail
IP="${PHONE_IP:-${1:-192.168.0.2}}"
PORT=5555

echo ">> checking for a USB-attached device..."
USB_SERIAL="$(adb devices | awk 'NR > 1 && $1 !~ /:/ && $2 == "device" { print $1; exit }')"
if [[ -z "$USB_SERIAL" ]]; then
  echo "   No USB device found. Plug the phone in via USB first, then re-run."
  exit 1
fi

echo ">> telling adbd to listen on tcp:${PORT} ..."
adb -s "$USB_SERIAL" tcpip "$PORT"
sleep 1

echo ">> connecting wirelessly to ${IP}:${PORT} ..."
adb connect "${IP}:${PORT}"

echo
echo ">> current devices:"
adb devices
echo
echo "✅ Done. You can unplug USB now. Wireless ADB is live until the next reboot."
