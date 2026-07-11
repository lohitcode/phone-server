#!/usr/bin/env bash
# phone-snap: take a photo with the phone's camera and pull it to the Mac.
#
# USAGE
#   ./phone-snap.sh             # saves ./remote.jpg
#   ./phone-snap.sh shot.jpg    # saves ./shot.jpg
#
# PREREQUISITE (one-time, on the phone)
#   Termux needs Camera permission: Settings → Apps → Termux → Permissions →
#   Camera → Allow. The first run will fail loudly if this isn't granted.
#   (termux-camera-photo talks to the Termux:API app, which must also be
#   Battery → Unrestricted like the rest of the Termux family.)

set -euo pipefail

OUT="${1:-remote.jpg}"

echo ">> snapping photo on phone → ~/${OUT} ..."
ssh phone "termux-camera-photo '${OUT}'"

echo ">> pulling to ./${OUT} ..."
scp "phone:${OUT}" "./${OUT}"

echo "✅ saved ./${OUT}"
