import { existsSync, readFileSync } from 'node:fs';
import { execFileSync, spawn } from 'node:child_process';
import { delimiter, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
const apiDir = join(root, 'apps', 'api');
const envFile = join(apiDir, '.env');

function loadEnv(file) {
  if (!existsSync(file)) return;

  for (const line of readFileSync(file, 'utf8').split(/\r?\n/)) {
    const match = line.match(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$/);
    if (!match || match[1] in process.env) continue;

    let value = match[2];
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    process.env[match[1]] = value;
  }
}

function commandExists(command) {
  try {
    execFileSync(process.platform === 'win32' ? 'where.exe' : 'which', [command], { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
}

loadEnv(envFile);

const goPath = execFileSync('go', ['env', 'GOPATH'], { encoding: 'utf8' }).trim();
const goBin = goPath ? join(goPath, 'bin') : '';
const pathEntries = (process.env.PATH ?? '').split(delimiter);
if (goBin && !pathEntries.includes(goBin)) pathEntries.push(goBin);
process.env.PATH = pathEntries.join(delimiter);

const useAir = commandExists('air');
const command = useAir ? 'air' : 'go';
const args = useAir ? [] : ['run', 'cmd/server/main.go'];

console.log(`Starting cpbridge API with ${useAir ? 'air' : 'go run'}...`);
const child = spawn(command, args, { cwd: apiDir, env: process.env, stdio: 'inherit' });

child.on('error', (error) => {
  console.error(`Failed to start API: ${error.message}`);
  process.exitCode = 1;
});

child.on('exit', (code, signal) => {
  process.exitCode = code ?? (signal ? 1 : 0);
});
