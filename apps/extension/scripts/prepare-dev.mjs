import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const extensionRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const devRoot = path.join(extensionRoot, '.dev');

fs.mkdirSync(devRoot, { recursive: true });
fs.copyFileSync(
  path.join(extensionRoot, 'manifest.development.json'),
  path.join(devRoot, 'manifest.json')
);

console.log(`Prepared development extension at ${devRoot}`);
