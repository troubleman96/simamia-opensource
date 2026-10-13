import { execFileSync, spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const [command, ...rawArgs] = process.argv.slice(2);
const args = [...rawArgs];
let target;

for (let i = 0; i < args.length; i += 1) {
  if (args[i] === '--target') {
    target = args[i + 1];
    args.splice(i, 2);
    break;
  }
}

function hostTarget() {
  return execFileSync('rustc', ['--print', 'host-tuple'], { encoding: 'utf8' }).trim();
}

function goTarget(targetTriple) {
  if (targetTriple.endsWith('-apple-darwin')) {
    return { goos: 'darwin', goarch: targetTriple.startsWith('aarch64-') ? 'arm64' : 'amd64' };
  }
  if (targetTriple.endsWith('-pc-windows-msvc')) {
    return { goos: 'windows', goarch: targetTriple.startsWith('aarch64-') ? 'arm64' : 'amd64' };
  }
  if (targetTriple.endsWith('-unknown-linux-gnu')) {
    return { goos: 'linux', goarch: targetTriple.startsWith('aarch64-') ? 'arm64' : 'amd64' };
  }
  throw new Error(`Unsupported desktop target triple: ${targetTriple}`);
}

function buildApi(targetTriple) {
  const { goos, goarch } = goTarget(targetTriple);
  const extension = goos === 'windows' ? '.exe' : '';
  const output = path.join(root, 'src-tauri', 'binaries', `simamia-api-${targetTriple}${extension}`);
  fs.mkdirSync(path.dirname(output), { recursive: true });
  const result = spawnSync(
    'go',
    ['build', '-trimpath', '-ldflags=-s -w', '-o', output, '.'],
    {
      cwd: path.join(root, 'api'),
      stdio: 'inherit',
      env: { ...process.env, CGO_ENABLED: '1', GOOS: goos, GOARCH: goarch }
    }
  );
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
  if (goos !== 'windows') fs.chmodSync(output, 0o755);
  console.log(`Built Go API sidecar: ${path.relative(root, output)}`);
}

if (!['dev', 'build', 'build-api'].includes(command)) {
  console.error('Usage: node scripts/desktop.mjs <dev|build|build-api> [--target <Rust target triple>]');
  process.exit(2);
}

const targetTriple = target || hostTarget();
buildApi(targetTriple);
if (command === 'build-api') process.exit(0);

const cli = path.join(root, 'node_modules', '.bin', process.platform === 'win32' ? 'tauri.cmd' : 'tauri');
if (!fs.existsSync(cli)) {
  console.error('Tauri CLI is missing. Run `bun install` first.');
  process.exit(1);
}
const tauriArgs = [command, ...(target ? ['--target', target] : []), ...args];
const result = spawnSync(cli, tauriArgs, {
  cwd: root,
  stdio: 'inherit',
  shell: process.platform === 'win32',
  env: process.env
});
if (result.error) throw result.error;
process.exit(result.status ?? 1);
