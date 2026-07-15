# Dedicated Android Cloud Instance

Turn an unused or broken-screen Android phone into a dedicated personal cloud
instance using Termux, SSH, and Cloudflare Tunnel. Develop workloads on your
personal computer, deploy them to the phone, and publish services over HTTPS
without opening a port on the router.

The phone becomes a reusable ARM64 compute node rather than a single-purpose
demo: it can host APIs, websites, scheduled jobs, databases, automation, and
monitoring tools within Android and Termux's limits. This repository includes a
small API workload as a working deployment example.

## **Important: network reliability is the biggest constraint**

This is a personal cloud instance running from your own network, so its uptime
depends on you. The phone must remain powered, connected to reliable Wi-Fi, and
reachable through a working router and internet connection. Cloudflare Tunnel
removes the need for a public IP or router port forwarding, but it cannot keep
the service online when the phone, Wi-Fi, router, power, or ISP connection is
down. Use monitoring and a conventional cloud provider when guaranteed uptime,
redundancy, or an SLA is required.

This repository currently powers:

- `GET /health` — basic service health
- `GET /api/v1/system` — cached CPU, RAM, swap, battery, storage, and uptime

Live example: [https://lohitcode.com/health](https://lohitcode.com/health)

## Project documentation

- [`README.md`](README.md) — start here for the complete, reproducible setup.
- [`DEPLOYMENT.md`](DEPLOYMENT.md) — deeper deployment architecture, operating
  trade-offs, persistence, rollback, and hardening notes.
- [`AGENTS.md`](AGENTS.md) — repository conventions and safety rules for Codex or
  other coding agents working on this project.

The scripts and paths documented in this README describe the current running
implementation. `DEPLOYMENT.md` also contains longer-term architecture ideas,
so prefer this README when a path or command differs.

## How it works

```text
Personal computer                    Android cloud instance
┌────────────────────┐               ┌─────────────────────────────┐
│ Develop workload   │               │ Termux                      │
│ Cross-compile      │  SSH/SCP      │ ├─ sshd :8022               │
│ ./phone-deploy.sh  ├──────────────►│ ├─ application :8080        │
└────────────────────┘               │ └─ cloudflared              │
                                     └──────────────┬──────────────┘
                                                    │ outbound tunnel
                                     ┌──────────────▼──────────────┐
Browser ─── HTTPS ──────────────────►│ Cloudflare edge + DNS       │
                                     └─────────────────────────────┘
```

`cloudflared` and the application run on the same phone, so the Cloudflare
origin is `http://127.0.0.1:8080`. No inbound router port forwarding is required.

## What you need

- An Android 7+ phone and a personal computer on the same Wi-Fi for initial setup
- A USB cable that supports data
- A domain using Cloudflare DNS
- [Homebrew](https://brew.sh/) or an equivalent package manager on the computer
- [Termux](https://github.com/termux/termux-app),
  [Termux:API](https://github.com/termux/termux-api), and
  [Termux:Boot](https://github.com/termux/termux-boot) on the phone

Install Termux and all its plugin apps from the **same source** (F-Droid or the
official GitHub releases). Android will reject or break plugins signed by a
different source.

On the development computer (Homebrew example):

```bash
brew install go scrcpy
brew install --cask android-platform-tools
```

References:

- [Android developer options](https://developer.android.com/studio/debug/dev-options)
- [Android Debug Bridge](https://developer.android.com/tools/adb)
- [Official scrcpy project](https://github.com/Genymobile/scrcpy)
- [Termux installation](https://github.com/termux/termux-app#installation)
- [Termux:Boot setup](https://github.com/termux/termux-boot#how-to-use)
- [Cloudflare Tunnel setup](https://developers.cloudflare.com/tunnel/setup/)

## 1. Prepare Android and ADB

### Enable developer options

1. Open **Settings → About phone**.
2. Tap **Build number** seven times.
3. Open **Settings → System → Developer options**. The exact path can vary by
   manufacturer.
4. Enable **USB debugging**.
5. Connect the phone to the computer over USB.
6. Accept the **Allow USB debugging** prompt on the phone. Select **Always allow
   from this computer** if this is your own trusted computer.

Verify the connection:

```bash
adb devices
```

The device must appear with the state `device`, not `unauthorized`.

### Mirror a working screen with scrcpy

```bash
scrcpy
```

For a phone with a broken display, the USB-debugging authorization must already
be trusted, or the display must be temporarily repaired/connected so the prompt
can be approved. scrcpy cannot bypass Android's lock screen or ADB authorization.

### Enable this project's wireless ADB mode

Reserve an IP address for the phone in the router first. This project currently
uses `192.168.0.2`; change `IP` in `phone-adb.sh` if your phone uses a different
address.

With USB connected:

```bash
./phone-adb.sh
adb devices
```

You can then unplug USB and use:

```bash
adb -s 192.168.0.2:5555 shell
scrcpy --serial 192.168.0.2:5555
```

This `adb tcpip` mode normally needs to be enabled again over USB after a phone
reboot. Port `5555` is trusted-LAN access: never forward it on the router or
publish it through Cloudflare.

## 2. Install and configure Termux

Open Termux and install the required packages:

```bash
pkg update && pkg upgrade
pkg install openssh termux-api cloudflared curl
```

The `termux-api` package provides the command-line client used by the metrics
endpoint. The separate Termux:API Android app must also be installed. Grant the
battery permission when Android asks for it.

Check the Termux username and the phone's Wi-Fi IP:

```bash
whoami
ip addr show wlan0
```

Example values used by this project:

```text
username: u0_a230
IP:       192.168.0.2
SSH port: 8022
```

### Start SSH and install a key

In Termux, create a temporary SSH password and start the daemon:

```bash
passwd
sshd
```

Termux SSH listens on port `8022`. On the development computer, create a key if
needed and copy it to the phone:

```bash
test -f ~/.ssh/id_ed25519 || ssh-keygen -t ed25519
ssh-copy-id -p 8022 YOUR_TERMUX_USERNAME@PHONE_IP
ssh -p 8022 YOUR_TERMUX_USERNAME@PHONE_IP
```

Add a convenient alias to `~/.ssh/config` on the development computer:

```sshconfig
Host phone
    HostName PHONE_IP
    Port 8022
    User YOUR_TERMUX_USERNAME
    IdentityFile ~/.ssh/id_ed25519
```

Test it:

```bash
ssh phone
```

After key authentication works, disable password login on the phone by setting
the following in `$PREFIX/etc/ssh/sshd_config`:

```text
PasswordAuthentication no
PubkeyAuthentication yes
```

Restart SSH from Termux:

```bash
pkill sshd
sshd
```

Keep the current Termux session open until a new `ssh phone` connection succeeds.

### Start SSH after reboot

Open the Termux:Boot Android app once after installing it. Then run in Termux:

```bash
mkdir -p ~/.termux/boot
cat > ~/.termux/boot/start-sshd <<'EOF'
#!/data/data/com.termux/files/usr/bin/sh
termux-wake-lock
sshd
EOF
chmod 700 ~/.termux/boot/start-sshd
```

In Android settings, set battery usage to **Unrestricted** for Termux,
Termux:API, and Termux:Boot. Do not expose SSH port `8022` through router port
forwarding.

## 3. Deploy the included workload

Clone this repository on the development computer:

```bash
git clone https://github.com/lohitcode/phone-server.git
cd phone-server
```

The included workload is implemented in Go so it can be cross-compiled into one
small ARM64 binary. Run it locally while developing:

```bash
go run .
curl http://127.0.0.1:8080/health
```

Run the tests and deploy:

```bash
go test ./...
./phone-deploy.sh
```

The deployment script:

1. Cross-compiles a static Android ARM64 binary with `CGO_ENABLED=0`.
2. Connects using the `ssh phone` alias.
3. Uploads the binary to `~/apps/phone-server`.
4. Stops the previous PID, swaps in the new binary, and starts it with `nohup`.
5. Verifies `http://127.0.0.1:8080/health` on the phone.

Useful commands:

```bash
# Verify through the phone itself
ssh phone 'curl --fail http://127.0.0.1:8080/health'

# Verify over the LAN
curl http://PHONE_IP:8080/health

# Follow application logs
ssh phone 'tail -f ~/apps/phone-server/phone-server.log'

# Override the SSH alias or remote directory
PHONE_HOST=my-phone PHONE_APP_DIR=apps/phone-server ./phone-deploy.sh
```

The application binds to `0.0.0.0:8080`. LAN access is intentional; router port
forwarding is not.

### Start the application after reboot

Create another Termux:Boot script on the phone:

```bash
cat > ~/.termux/boot/start-phone-server <<'EOF'
#!/data/data/com.termux/files/usr/bin/sh
termux-wake-lock

APP_DIR="$HOME/apps/phone-server"
cd "$APP_DIR" || exit 0

if [ -x ./phone-server ] && ! pgrep -f '[p]hone-server' >/dev/null; then
  nohup ./phone-server >> phone-server.log 2>&1 < /dev/null &
  echo $! > phone-server.pid
fi
EOF
chmod 700 ~/.termux/boot/start-phone-server
```

Test it without rebooting:

```bash
~/.termux/boot/start-phone-server
curl http://127.0.0.1:8080/health
```

## 4. Create a Cloudflare Tunnel

This project uses a **remotely managed tunnel**. Its hostname-to-origin routing
is stored in Cloudflare, while the tunnel token stays on the phone.

### Create the tunnel in the dashboard

1. Sign in to the [Cloudflare dashboard](https://dash.cloudflare.com/).
2. Ensure the domain you want to use is active in Cloudflare DNS.
3. Go to **Networking → Tunnels**.
4. Select **Create a tunnel**.
5. Choose **Cloudflared** and give it a name, such as `android-cloud`.
6. Continue to the connector installation screen.
7. The dashboard may show commands for several desktop/server operating systems
   or Docker.
   Do **not** run the Debian/Red Hat `sudo` commands in Termux. Termux is Android,
   and `cloudflared` was already installed with `pkg`.
8. Copy only the tunnel token from the displayed
   `cloudflared tunnel run --token ...` command. Treat it like a password.

If a tunnel token is pasted into chat, logs, or a public repository, use
**Rotate token** in the tunnel dashboard and replace it immediately.

### Store the token locally and configure the phone

Create a gitignored `.env` in the project root on the development computer:

```bash
printf 'CLOUDFLARE_TUNNEL_TOKEN=%s\n' 'PASTE_NEW_TOKEN_HERE' > .env
chmod 600 .env
./setup-cloudflare-tunnel.sh
```

The setup script securely copies the token to
`~/.config/cloudflared/tunnel-token`, installs
`~/.termux/boot/start-cloudflared`, starts the connector, and writes logs to
`~/.config/cloudflared/cloudflared.log`.

Confirm that it is connected:

```bash
ssh phone "pgrep -af '[c]loudflared tunnel run'"
ssh phone 'tail -f ~/.config/cloudflared/cloudflared.log'
```

The Cloudflare dashboard should show the tunnel as **Healthy** with an
`android_arm64` connector.

Cloudflare documents token files for remotely managed tunnels in its
[tunnel run parameters](https://developers.cloudflare.com/tunnel/advanced/run-parameters/#token-file).

## 5. Assign the domain and port in Cloudflare

Starting a connector is not enough; the tunnel also needs a published
application route:

1. In **Networking → Tunnels**, open the tunnel.
2. Open **Routes** and select **Add route**.
3. Choose **Published application**.
4. Under **Hostname**, choose your Cloudflare domain. Enter a subdomain such as
   `phone` for `phone.example.com`, or leave the subdomain empty to use the apex
   domain when the dashboard permits it.
5. Set **Service type** to `HTTP`.
6. Set **Service URL** to:

   ```text
   http://127.0.0.1:8080
   ```

7. Save the route.

Cloudflare will create/manage the DNS record that points the hostname to the
tunnel. The browser connects to Cloudflare over HTTPS; Cloudflare forwards the
request through the encrypted tunnel to port `8080` on the phone.

Official reference: [Publish an application through Cloudflare Tunnel](https://developers.cloudflare.com/tunnel/setup/#publish-an-application).

Verify from any network:

```bash
curl https://YOUR_DOMAIN/health
curl https://YOUR_DOMAIN/api/v1/system
```

Expected health response:

```json
{"status":"ok","timestamp":"2026-07-15T00:00:00Z"}
```

## 6. Live metrics endpoint

`GET /api/v1/system` returns a public, recruiter-safe subset of phone metrics:

```json
{
  "status": "ok",
  "timestamp": "2026-07-15T00:00:00Z",
  "uptime": "2 hours, 10 minutes",
  "cpu": {"cores": 8, "usage_percent": 0.3},
  "memory": {
    "total_mb": 5416,
    "available_mb": 2500,
    "used_percent": 53.8,
    "swap_total_mb": 6143,
    "swap_used_mb": 1100
  },
  "battery": {
    "percentage": 90,
    "status": "DISCHARGING",
    "health": "GOOD",
    "temperature_celsius": 34
  },
  "storage": {
    "total_gb": 108,
    "available_gb": 95.8,
    "used_percent": 11
  },
  "freshness": {
    "cpu_memory": "2026-07-15T00:00:00Z",
    "battery_storage": "2026-07-15T00:00:00Z"
  }
}
```

CPU and memory are sampled every 5 seconds. Battery, storage, and uptime are
sampled every 30 seconds. Requests read the cached snapshot instead of launching
commands. Android prevents an ordinary Termux app from reading every
system-wide CPU counter, so CPU usage represents the Termux-visible workload
normalized across the phone's cores.

## Troubleshooting

### `adb devices` shows `unauthorized`

Unlock the phone and accept the debugging prompt. If necessary, use **Developer
options → Revoke USB debugging authorizations**, reconnect USB, and approve the
computer again.

### Wireless ADB stopped working

Connect USB and rerun `./phone-adb.sh`. Also confirm the phone still has the IP
configured in the script.

### `ssh phone` says `No route to host`

- Confirm the development computer and phone are on the same LAN.
- Check the router reservation and the `HostName` in `~/.ssh/config`.
- Open Termux through ADB/scrcpy and run `sshd`.
- Test the port with `nc -vz PHONE_IP 8022`.

The deployment script deliberately uses `ssh phone`, not a hard-coded SSH
command, so the alias remains the source of truth.

### Deployment health check fails

```bash
ssh phone 'cat ~/apps/phone-server/phone-server.log'
ssh phone 'cat ~/apps/phone-server/phone-server.pid'
ssh phone 'curl -v http://127.0.0.1:8080/health'
```

Make sure no other process occupies port `8080`.

### Tunnel is Healthy but the hostname returns `404`

The connector is running but no matching route exists. Add a **Published
application** route whose hostname matches the requested domain and whose
service is `http://127.0.0.1:8080`.

### Cloudflare returns `502 Bad Gateway`

Cloudflare can reach the connector, but the connector cannot reach the
application. Deploy/start the application and verify its local health URL from
the phone.

### `A DNS record managed by Workers already exists on that host`

That hostname is already assigned to a Cloudflare Worker route/custom domain.
Remove the conflicting Worker mapping in Cloudflare or publish this tunnel on a
different subdomain, then add the tunnel route again.

### Services do not start after reboot

- Open Termux:Boot once after installation.
- Set Termux, Termux:API, and Termux:Boot battery usage to **Unrestricted**.
- Confirm scripts exist and are executable with `ls -l ~/.termux/boot`.
- Run each boot script manually and inspect its log before reboot testing.

## Security notes

- Never commit `.env`, tunnel tokens, SSH keys, or device data.
- Never expose ADB `5555`, SSH `8022`, or the application port `8080` using
  router port forwarding.
- Use SSH keys and disable SSH password authentication after verifying the key.
- Keep public API responses aggregate and non-sensitive. Do not add arbitrary
  shell execution, process lists, environment variables, IP addresses, or device
  identifiers to HTTP endpoints.
- Add [Cloudflare Access](https://developers.cloudflare.com/cloudflare-one/access-controls/) before publishing private dashboards or administrative APIs.

## Repository scripts

| Script | Purpose |
| --- | --- |
| `phone-adb.sh` | Enables wireless ADB on `PHONE_IP:5555` after a USB connection |
| `phone-deploy.sh` | Cross-compiles, uploads, restarts, and health-checks the included workload |
| `setup-cloudflare-tunnel.sh` | Installs the tunnel token and Termux:Boot launcher |
| `phone-snap.sh` | Takes a photo through Termux:API and copies it to the development computer |

## License

Add a license before reusing this project beyond personal experimentation.
