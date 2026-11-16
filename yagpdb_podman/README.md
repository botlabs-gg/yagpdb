# YAGPDB Rootless Podman Quadlet Setup

This directory provides systemd Quadlet unit files for running YAGPDB rootlessly with Podman, PostgreSQL, and Valkey.

## Directory Layout

* `quadlet/` — Podman Quadlet definitions (`.container`, `.build`, `.network`) and example environment files (`app.example.env`, `db.example.env`).
* `systemd/` — Optional `podman-auto-update.service` drop-in so scheduled container updates automatically rebuild the local YAGPDB image.

## Prerequisites

* **Podman 5.2+** (required for `.build` Quadlet units)
* **User lingering enabled** so rootless containers stay running after logout:
  ```bash
  loginctl enable-linger "$USER"
  ```

## Installation

1. **Copy the Quadlet files to your user systemd directory:**
   ```bash
   mkdir -p ~/.config/containers/systemd
   cp -r quadlet/* ~/.config/containers/systemd/
   ```

2. **Configure your environment variables:**
   Rename the example `.env` files (removing `.example`) and fill in your Discord application credentials, host domain, and database password:
   ```bash
   mv ~/.config/containers/systemd/app.example.env ~/.config/containers/systemd/app.env
   mv ~/.config/containers/systemd/db.example.env ~/.config/containers/systemd/db.env
   ```
   *(Make sure `EnvironmentFile=` in `yagpdb-app.container` and `yagpdb-postgres.container` points to `app.env` and `db.env`.)*

3. **Reload systemd and build the YAGPDB image:**
   `yagpdb.build` builds directly from upstream Git using `yagpdb_docker/Dockerfile`. Build the image once before starting the application service:
   ```bash
   systemctl --user daemon-reload
   systemctl --user start yagpdb-build.service
   ```

4. **Start the stack:**
   ```bash
   systemctl --user start yagpdb-postgres.service yagpdb-valkey.service yagpdb-app.service
   ```
   *(Note: Quadlet-generated services are transient units enabled automatically at boot via their `[Install]` sections after `daemon-reload`.)*

## Automatic Updates (Optional)

`yagpdb-app.container` uses `After=yagpdb-build.service` (ordering-only) rather than `Requires=`, so manual restarts (`systemctl --user restart yagpdb-app.service`) happen immediately without rebuilding the image.

To trigger image rebuilds automatically when Podman's auto-update timer runs:

1. **Install the `podman-auto-update.service` drop-in:**
   ```bash
   mkdir -p ~/.config/systemd/user
   cp -r systemd/* ~/.config/systemd/user/
   systemctl --user daemon-reload
   ```

2. **Enable Podman's auto-update timer:**
   ```bash
   systemctl --user enable --now podman-auto-update.timer
   ```

When the timer fires, systemd runs `yagpdb-build.service` first and then `podman-auto-update.service` recreates `yagpdb-app.service` (alongside Postgres and Valkey) whenever a newer image digest is built or pulled.