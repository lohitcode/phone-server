# Phone Server — Codex Project Guide

## Project

This repository contains a small Go HTTP service deployed to a headless CMF Phone 1
running Termux. The Mac is the development and build machine; the phone is the
deployment target.

Read `DEPLOYMENT.md` before changing the deployment or remote-access setup.

## Development

- Go module: `github.com/lohit/phone-server`
- Default listen address: `127.0.0.1:8080`
- Override the port with the `PORT` environment variable.
- Health endpoint: `GET /health`
- Format changes with `gofmt`.
- Run `go test ./...` before deployment.

## Deployment

- Deploy from the Mac with `./deploy-phone.sh`.
- The script cross-compiles with `GOOS=android`, `GOARCH=arm64`, and `CGO_ENABLED=0`.
- Connect to the phone only through the existing `ssh phone` alias.
- The remote application directory is `~/apps/phone-server`.
- Verify the deployed service from the phone with:

  ```bash
  ssh phone 'curl --fail http://127.0.0.1:8080/health'
  ```

## Device access

- SSH alias: `phone`
- Termux SSH port: `8022`
- Wireless ADB helper: `./phone-adb.sh`
- Camera helper: `./phone-snap.sh`
- Wireless ADB may need to be enabled again after a reboot.

## Safety

- Keep the Go service bound to `127.0.0.1`; expose it later through Cloudflare
  Tunnel instead of router port forwarding.
- Never expose SSH, ADB, or Termux command execution through the HTTP service.
- Do not commit secrets, Cloudflare credentials, device data, database files, or
  private keys.
- Do not reboot the phone, change its SSH configuration, or alter Termux boot
  scripts unless the user explicitly requests it.
- Preserve the PID/log-based restart behavior in `deploy-phone.sh` unless replacing
  it with a verified service manager.

## Device notes

- Device: CMF Phone 1 (Nothing), Android 15, ARM64
- Termux user: `u0_a230`
- The router currently reserves `192.168.0.2` for the phone.
- Termux and Termux:Boot need unrestricted battery access so `sshd` remains alive.
- Termux:Boot starts SSH using `~/.termux/boot/start-sshd`.
