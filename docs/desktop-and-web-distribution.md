# Desktop and web distribution

Simamia keeps a single Next.js interface for browser and desktop. The desktop shell wraps the existing static export; it does not introduce a second UI or a separate set of components.

## Desktop app

The Tauri v2 app bundles `out/` with the Go API as a platform-specific sidecar. The sidecar uses SQLite, binds only to `127.0.0.1:8080`, permits the desktop WebView origins used by Tauri, and is stopped when the desktop app exits. The database is stored in Tauri's per-user app-data directory under the identifier `online.simamia.desktop`.

### Prerequisites

- Bun 1.x
- Go version declared in `api/go.mod`
- Rust 1.90 or newer and Cargo
- A native C compiler because the current Go SQLite driver uses CGO
- Tauri's system dependencies: see the [Tauri prerequisites](https://v2.tauri.app/start/prerequisites/)

On Windows, use a MinGW-w64 GCC toolchain for the Go API build. macOS and Linux require their normal Xcode/Clang or GCC toolchains.

### Develop and build

```bash
bun install
bun run desktop:dev
bun run desktop:build
```

`desktop:dev` compiles the API for the current Rust host target, starts the Next.js dev server, and opens the desktop shell. `desktop:build` compiles the sidecar and bundles a native installer. For a non-host target, build on the matching operating system/architecture and pass the target triple through the script, for example `node scripts/desktop.mjs build --target aarch64-apple-darwin`.

The `.github/workflows/desktop-release.yml` workflow builds and drafts installers when a `v*` tag is pushed: Linux x64, Windows x64, and macOS x64 plus Apple Silicon. Linux bundles all formats supported by Tauri on the runner; Windows and macOS bundle their native installer formats. The workflow intentionally drafts releases, so a maintainer can review the packages before publishing them.

### Authentication configuration

Sign-in remains Camel Accounts OAuth. Tauri changes the app origin, so the OAuth client used by the release must allow these callback URLs:

- `tauri://localhost/auth/callback/`
- `http://tauri.localhost/auth/callback/`
- `http://localhost:3000/auth/callback/` for desktop development

Set the repository Actions variables `NEXT_PUBLIC_CAMEL_ACCOUNTS_URL` and `NEXT_PUBLIC_CAMEL_ACCOUNTS_CLIENT_ID` if the release should use a different public OAuth client. These are public client settings, not secrets. The account flow and API token validation require an internet connection; the local SQLite database does not sync between devices automatically.

The desktop API bind address is configurable via `SIMAMIA_LISTEN_ADDRESS`; its default remains `0.0.0.0` for existing Docker/server deployments. Desktop startup explicitly overrides it to loopback.

## Static browser build

The existing Next.js static export remains available and can be hosted by any static file server:

```bash
NEXT_PUBLIC_CAMEL_ACCOUNTS_URL=https://accounts.camelcreatives.com \
NEXT_PUBLIC_CAMEL_ACCOUNTS_CLIENT_ID=your-public-client-id \
NEXT_PUBLIC_SIMAMIA_API_URL=https://your-api.example/api/v1 \
bun run build
```

Serve the generated `out/` directory. The API must be reachable from the browser, validate Camel Accounts tokens, and allow the deployed app's origin in `CORS_ALLOWED_ORIGINS`.

## Self-hosted Docker

The existing Compose setup continues to build and serve the public site, static app, API, admin console, and API docs. Start from `.env.server.example`, set deployment-specific values and secrets, then run `docker compose up --build`. Do not expose the API database or commit populated database files or real environment secrets.
