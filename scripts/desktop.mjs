import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const desktopDir = path.join(root, 'desktop');
const [command, ...rawArgs] = process.argv.slice(2);
const args = [...rawArgs];
let platform;

for (let i = 0; i < args.length; i += 1) {
  if (args[i] === '--platform' || args[i] === '--target') {
    platform = args[i + 1];
    args.splice(i, 2);
    break;
  }
}

const legacyTargets = {
  'x86_64-unknown-linux-gnu': 'linux/amd64',
  'aarch64-unknown-linux-gnu': 'linux/arm64',
  'x86_64-pc-windows-msvc': 'windows/amd64',
  'aarch64-pc-windows-msvc': 'windows/arm64',
  'x86_64-apple-darwin': 'darwin/amd64',
  'aarch64-apple-darwin': 'darwin/arm64'
};
platform = legacyTargets[platform] || platform;

function run(file, commandArgs, cwd, env = process.env) {
  const result = spawnSync(file, commandArgs, {
    cwd,
    stdio: 'inherit',
    env,
    shell: process.platform === 'win32'
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

function prepareFrontend() {
  const next = path.join(root, 'node_modules', 'next', 'dist', 'bin', 'next');
  if (!fs.existsSync(next)) {
    throw new Error('Next.js is not installed. Install dependencies with `bun install` first.');
  }
  const env = {
    ...process.env,
    NEXT_PUBLIC_SIMAMIA_DESKTOP: 'true',
    NEXT_PUBLIC_SIMAMIA_API_URL: '/api/v1',
    NEXT_PUBLIC_SENTRY_DISABLED: 'true'
  };

  const result = spawnSync(process.execPath, [next, 'build'], {
    cwd: root,
    stdio: 'inherit',
    env
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);

  const builtExport = path.join(root, 'out');
  const embeddedExport = path.join(desktopDir, 'frontend', 'out');
  if (!fs.existsSync(path.join(builtExport, 'index.html'))) {
    throw new Error('Next.js did not produce out/index.html; the Wails app cannot embed the UI.');
  }
  fs.rmSync(embeddedExport, { recursive: true, force: true });
  fs.cpSync(builtExport, embeddedExport, { recursive: true });
}

function runWails() {
  const wailsArgs = [command, ...(platform ? ['-platform', platform] : []), ...args];
  const cli = process.env.WAILS_CLI || (process.platform === 'win32' ? 'wails.exe' : 'wails');
  const result = spawnSync(cli, wailsArgs, {
    cwd: desktopDir,
    stdio: 'inherit',
    env: process.env,
    shell: process.platform === 'win32'
  });
  if (result.error?.code === 'ENOENT') {
    run('go', ['run', 'github.com/wailsapp/wails/v2/cmd/wails@v2.16.0', ...wailsArgs], desktopDir);
    return;
  }
  if (result.error) throw result.error;
  process.exit(result.status ?? 1);
}

if (!['dev', 'build'].includes(command)) {
  console.error('Usage: node scripts/desktop.mjs <dev|build> [--platform <linux/amd64|windows/amd64|darwin/universal>] [Wails flags]');
  process.exit(2);
}

prepareFrontend();
runWails();
