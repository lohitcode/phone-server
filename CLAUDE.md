# CMF Phone 1 - Development Server Setup

> **Device**: CMF Phone 1 (Nothing)
> **Purpose**: Portable Linux development server accessible via SSH
> **IP**: 192.168.0.2 (static DHCP reserved)
> **SSH**: `ssh phone` (from Mac)
> **Strategy**: see [DEPLOYMENT.md](DEPLOYMENT.md) — build/ship/run + Cloudflare Tunnel for remote access

---

## ✅ Completed Steps

### 1. Installed Termux
Linux user-space environment on Android.

### 2. Installed Termux:Boot
Enables automatic service startup after reboot.

### 3. Updated Termux packages
```bash
pkg update
pkg upgrade -y
```

### 4. Installed OpenSSH
```bash
pkg install openssh
```

### 5. Started SSH server
```bash
sshd
```

### 6. Set Termux password
```bash
passwd
```

### 7. Confirmed phone IP address
Found via ADB: `192.168.0.2`

### 8. SSH from Mac
```bash
ssh -p 8022 u0_a230@192.168.0.2
```
Successfully connected after password authentication.

### 9. Set up SSH config alias on Mac
Created `~/.ssh/config`:
```ssh
Host phone
    HostName 192.168.0.2
    User u0_a230
    Port 8022
```
Now connects with just `ssh phone`.

### 10. Configured wireless ADB
```bash
adb tcpip 5555
adb connect 192.168.0.2:5555
```
Can now debug wirelessly without USB.

### 11. Reserved static IP on router (TP-Link)
- MAC: `22:3B:00:F7:E7:3D`
- IP: `192.168.0.2`
- Device name: `CMF-by-Nothing-Phone-1`

The IP is now reserved and won't change after router reboots.

---

## 🔜 Next Steps

### Priority
- ✅ Verify SSH username → `u0_a230` (`whoami`)
- ✅ Set up SSH key authentication → ed25519 key installed; `ssh phone` logs in passwordless
- ✅ Configure automatic SSH startup with Termux:Boot → `~/.termux/boot/start-sshd` runs `termux-wake-lock` + `sshd`
- ✅ Test connection after phone reboot → sshd auto-started and **stayed up** (verified stable)

> **Persistence requirement (Nothing OS):** both **Termux** and **Termux:Boot** must be set to *Battery → Unrestricted* (doze whitelist), or the OS kills Termux and sshd dies after boot.

### Development Environment (native Termux — no proot) ✅
> **Decision:** Go compiles to a static binary and doesn't need glibc, so `proot-distro`
> only adds RAM/CPU tax on a 6 GB device. Run everything native in Termux. Install
> Debian via proot only later if some tool strictly requires glibc.

- ✅ Dev tools via `pkg`: `golang` (1.26.3), `git` (2.55.0), `helix` (25.07.1), `tmux` (3.7b), `sqlite` (3.53.3)
- ✅ Editor: **Helix** (`hx`) — Neovim removed
- ✅ CGO ready: `clang` 21.1.8 already installed, so `go-sqlite3` builds natively; `sqlite3` CLI on hand to inspect DBs
- ✅ Smoke test: `go run hello.go` on-device → "hello from phone-server"
- ✅ Workspace: `~/workspace` created; GOPATH defaults to `~/go`

---

## Quick Reference

### SSH Connection
```bash
ssh phone
```

### ADB (Wireless — does NOT survive reboot on non-root)
After each reboot, re-trigger once over USB, then go wireless:
```bash
./phone-adb.sh     # plug USB in first; flips adbd to tcp:5555, then connects 192.168.0.2:5555
```
Day-to-day, prefer `ssh phone`. ADB is only for APK installs / logcat / recovery.

### Camera (remote shutter → pull to Mac)
```bash
./phone-snap.sh             # saves ./remote.jpg
./phone-snap.sh shot.jpg    # custom filename
```
Uses `termux-camera-photo`; needs Termux **Camera** permission granted once.

### Boot sound (QoL)
On every boot, `~/.termux/boot/start-sshd` fires `termux-notification --sound` → "phone-server: sshd up". Audible + visible confirmation the server came up.

### Device Info
- **User**: `u0_a230`
- **Port**: `8022`
- **IP**: `192.168.0.2`
- **MAC**: `22:3B:00:F7:E7:3D`

---

## Project Context

This phone is being set up as a portable development server that can be:
- Accessed from any Mac on the local network
- Used for development, testing, and deployment
- Always reachable via the reserved IP and SSH alias
- Maintained with minimal overhead (no desktop Linux VM needed)
