# Phone-Server Deployment Strategy

> Dev on Mac → ship binary → run on phone (native Termux) → reachable on **LAN** and **remotely via Cloudflare Tunnel**.

## Architecture

```
┌────────────── Mac / any device, from anywhere ───────────┐
│  ssh phone-tunnel   →  cloudflared access (ProxyCommand)  │
│  curl https://api.yourdomain.com/health                   │
└─────────────────┬─────────────────────────────────────────┘
                  │ Cloudflare edge (Zero Trust + Access)
                  │ outbound-only tunnel (QUIC / HTTP2)
┌──────────────────▼──────────────────── Phone 192.168.0.2 ─┐
│  Termux (native, no proot)                                 │
│   ├─ sshd            :8022   (LAN: `ssh phone`)            │
│   ├─ Go server       :8080   ~/srv/server + data/app.db    │
│   └─ cloudflared     → outbound tunnel to CF edge          │
│        ingress:                                            │
│          ssh.yourdomain.com  → ssh://localhost:8022         │
│          api.yourdomain.com  → http://localhost:8080        │
│  All three auto-start via Termux:Boot + termux-wake-lock   │
└────────────────────────────────────────────────────────────┘
```

**Principles**
- Source + builds on Mac; only binaries + DB live on the phone.
- The tunnel is **outbound-only**: no router port-forwarding, no public IP, works on any Wi-Fi the phone joins.

---

## Part A — Build & ship (Mac → phone)

### 1. Build (Mac)
Pure-Go only. DB driver: **`modernc.org/sqlite`** (pure Go) — ❌ not `mattn/go-sqlite3` (CGO → needs Android NDK for cross-compile).
```bash
CGO_ENABLED=0 GOOS=android GOARCH=arm64 \
  go build -trimpath -ldflags="-s -w" -o bin/server .
```
- `CGO_ENABLED=0` → clean cross-compile
- `-trimpath` → reproducible, strips Mac paths
- `-ldflags="-s -w"` → strip debug → ~5–6 MB (from 8)

### 2. Phone layout (keep deploy separate from scratch)
```
~/srv/server        # deployed binary
~/srv/data/app.db   # SQLite DB (persistent, lives on phone)
~/srv/server.log
~/srv/server.pid
~/cloudflared/      # tunnel creds + config.yml
~/workspace/        # scratch / on-device experiments
```

### 3. Deploy script — `phone-deploy.sh` (Mac)
`build (strip) → stop old (pidfile) → backup → scp → start detached → health-check`
- Stop old: `kill $(cat ~/srv/server.pid)` — **pidfile**, not `pkill` by path (argv[0] is `./server`).
- Start: `setsid ./server </dev/null >>server.log 2>&1 & echo $! >server.pid` — `setsid` avoids the SSH-hang gotcha.

---

## Part B — Run & persist (phone)

### 4. Termux:Boot restart-loop (survives reboot + crashes)
`~/.termux/boot/start-server`:
```sh
#!/data/data/com.termux/files/usr/bin/sh
termux-wake-lock
cd ~/srv
while :; do
  ./server >>server.log 2>&1
  echo "$(date) exited — restarting in 2s" >>server.log
  sleep 2
done
```
Upgrade path: **`termux-services`** (runit) → `sv status|restart server` + log rotation. Add only when needed.

### 5. Binding
Bind **`:8080`** (`0.0.0.0`), not `127.0.0.1` — required for LAN reach (cloudflared hits localhost anyway, but LAN curl needs 0.0.0.0).

### 6. SQLite
- WAL mode: `PRAGMA journal_mode=WAL`.
- **Backup from the Mac on a schedule** (phone = single point of failure): `scp phone:~/srv/data/app.db ./backups/` or `sqlite3 app.db ".backup './backups/app.db'"`.

### 7. Rollback
`cp ~/srv/server ~/srv/server.bak` before every deploy; on failure → `cp server.bak server` + restart.

---

## Part C — Remote access via Cloudflare Tunnel 🆕

Lets you `ssh` into the phone and hit the Go API from **anywhere**, not just your LAN. Outbound-only — no port forwarding, works on any Wi-Fi the phone joins.

### 8. Prerequisites
- A **Cloudflare account** + a **domain on Cloudflare DNS**.
- **Zero Trust** (free tier covers personal use).

### 9. One-time setup (on the Mac — browser auth is easy there)
```bash
brew install cloudflared
cloudflared tunnel login                      # browser auth, picks your domain
cloudflared tunnel create phone               # → ~/.cloudflared/<UUID>.json
cloudflared tunnel route dns phone ssh.yourdomain.com
cloudflared tunnel route dns phone api.yourdomain.com
```
Write `config.yml`:
```yaml
tunnel: <UUID>
credentials-file: /data/data/com.termux/files/home/cloudflared/<UUID>.json
ingress:
  - hostname: ssh.yourdomain.com
    service: ssh://localhost:8022
  - hostname: api.yourdomain.com
    service: http://localhost:8080
  - service: http_status:404
```

### 10. Install cloudflared on the phone (native, no proot)
cloudflared is Go → build it natively in Termux:
```bash
pkg install golang                                 # already installed
CGO_ENABLED=0 go install github.com/cloudflare/cloudflared/cmd/cloudflared@latest
# → ~/go/bin/cloudflared
```
Fallback if the native build errors: run the prebuilt `cloudflared-linux-arm64` under `proot-distro debian` (the one sanctioned proot use).

### 11. Copy creds to the phone + run
```bash
# from Mac
scp ~/.cloudflared/<UUID>.json phone:~/cloudflared/
scp config.yml phone:~/cloudflared/
```
Test once: `ssh phone '~/go/bin/cloudflared tunnel --config ~/cloudflared/config.yml run phone'`

### 12. Protect with Cloudflare Access (essential — don't skip)
Zero Trust → Access → Applications → add both hostnames with a policy allowing **only your email/SSO**. Without this, anyone with the URL reaches your SSH/API.

### 13. Run cloudflared persistently
`~/.termux/boot/start-cloudflared`:
```sh
#!/data/data/com.termux/files/usr/bin/sh
termux-wake-lock
cd ~/cloudflared
while :; do
  ~/go/bin/cloudflared tunnel --config ~/cloudflared/config.yml run phone >>cloudflared.log 2>&1
  echo "$(date) cloudflared exited — restarting" >>cloudflared.log
  sleep 3
done
```

### 14. Remote SSH from the Mac (native ssh)
`brew install cloudflared` (already done above). Add to `~/.ssh/config`:
```
Host phone-tunnel
    HostName ssh.yourdomain.com
    ProxyCommand /opt/homebrew/bin/cloudflared access ssh --hostname %h
    User u0_a230
```
Then `ssh phone-tunnel` works from anywhere (Access SSO prompt on first use, token cached). Keep `ssh phone` for fast LAN access.

### 15. Remote API
`curl https://api.yourdomain.com/health` — also behind Access; use a **service token** for unattended/machine access.

---

## Part D — Hardening (now that SSH is internet-reachable)

- 🔴 **Disable password auth, key-only** — `~/.ssh/sshd_config`: `PasswordAuthentication no`. (Was optional before, **mandatory** now.)
- Cloudflare Access policy = the real front door; keep it to your identity only.
- Strong ed25519 key (already set).
- Keep cloudflared updated: `go install ...@latest`.

---

## Part E — Costs / trade-offs
- Tunnel adds ~20–80 ms latency vs LAN. SSH + Helix over it is usable, just laggier than LAN.
- cloudflared is a 3rd always-on process → some extra battery drain (mitigated by Unrestricted battery + wake-lock).
- Works on **any Wi-Fi** the phone joins (portable!) — the public hostname is stable regardless of the phone's IP.
- Free tier covers all of this.

### Alternative (lighter, no public exposure)
If you don't need a public URL, **Tailscale** gives point-to-point remote SSH with zero public exposure and no domain — easier/safer for pure "my devices talk to each other." Cloudflare Tunnel is the right call if you want a stable public hostname + API access.

---

## Persistence checklist
- ✅ Termux **and** Termux:Boot → *Battery → Unrestricted* (doze whitelist)
- ✅ `termux-wake-lock` in every boot script (sshd, server, cloudflared)
- ✅ Static IP on LAN (192.168.0.2)
- ⬜ Password auth disabled (key-only) — do before/with enabling the tunnel
