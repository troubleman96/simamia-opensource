# Desktop and web distribution

Simamia keeps one Next.js interface for browser, Docker, and desktop. Wails embeds the existing static export; the desktop does not introduce a second frontend or a separate component set.

## Wails desktop app

The Wails v2 desktop app bundles the static UI and mounts the existing Go API handler inside Wails's local asset server. No API sidecar, listening TCP port, Rust runtime, or internet connection is required for normal local workspace use. The database is stored at the per-user configuration path reported by the OS (`Simamia/simamia.db` under `os.UserConfigDir()`):

- Linux: typically `~/.config/Simamia/simamia.db`
- Windows: typically `%AppData%\Simamia\simamia.db`
- macOS: typically `~/Library/Application Support/Simamia/simamia.db`

The desktop build uses a local-only workspace identity and SQLite persistence. The visible dashboard and business onboarding use the same React components and styles as the browser app. The desktop does not authenticate against Camel Accounts and does not synchronize its local records with the web app or other devices. Keep a file-level backup of the SQLite database before moving, reinstalling, or replacing a device.

### Online-only boundaries

Browser and Docker builds continue to use Camel Accounts OAuth and the existing remote API configuration. On desktop, internet-only operations are not needed to open the UI or create, read, update, and delete local business records. Cross-device sync, Camel account registration/recovery, Google sign-in, and future SMS, email, AI-provider, and mobile-money integrations require network connectivity and are not provided by this local workspace. The operational payments screens remain ledger entries, not payment processing. Local desktop records are protected by the OS user account; the app currently has no additional password, encryption, or multi-user access-control layer.

### Prerequisites

- Bun 1.x (or the installed project dependencies), Go 1.26+, and Wails v2
- A C toolchain for the SQLite driver
- Linux builds need GTK 3 and WebKitGTK 4.1 development packages plus `pkg-config`; the packaged app also needs the GTK/WebKitGTK shared libraries installed on the target computer
- Windows: WebView2 runtime and a C compiler for SQLite
- macOS: Xcode command-line tools

See the [Wails installation guide](https://wails.io/docs/gettingstarted/installation/) for platform setup.

### Develop and build

```bash
bun install
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
bun run desktop:dev
bun run desktop:build
```

Both commands build the Next.js static export with desktop-local settings and copy it to `desktop/frontend/out` before invoking Wails. `desktop:dev` serves the embedded files from disk in Wails's dev window; rerun it after changing the frontend. To build for a specific platform, pass a Wails platform, for example `bun run desktop:build --platform linux/amd64` or `bun run desktop:build --platform darwin/universal`. Native packaging requires building on a compatible host OS.

The `.github/workflows/desktop-release.yml` workflow builds Linux x64, Windows x64, and a universal macOS application when a `v*` tag is pushed. It creates a draft release for review before publishing.

## Static browser build

The existing browser app remains a static export and continues to use the public Camel Accounts client and configured API:

```bash
NEXT_PUBLIC_CAMEL_ACCOUNTS_URL=https://accounts.camelcreatives.com \
NEXT_PUBLIC_CAMEL_ACCOUNTS_CLIENT_ID=your-public-client-id \
NEXT_PUBLIC_SIMAMIA_API_URL=https://your-api.example/api/v1 \
bun run build
```

Serve the generated `out/` directory. The API must be reachable from the browser, validate Camel Accounts tokens, and allow the deployed app's origin in `CORS_ALLOWED_ORIGINS`.

## Self-hosted Docker

The existing Compose setup continues to build and serve the public site, static app, API, admin console, and API docs. Start from `.env.server.example`, set deployment-specific values and secrets, then run:

```bash
cp .env.server.example .env
docker compose up --build
```

The Docker and browser distribution paths are independent of the Wails desktop wrapper. Do not expose the API database or commit populated database files or real environment secrets.
