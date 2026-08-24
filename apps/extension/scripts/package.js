import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const { ZipArchive } = require('archiver');

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const extensionRoot = path.resolve(__dirname, '..');
const distDir = path.join(extensionRoot, 'dist');
const manifestFile = path.join(extensionRoot, 'manifest.json');

const webStaticDownloadsDir = path.resolve(extensionRoot, '../web/static/downloads');
const outputZipInWeb = path.join(webStaticDownloadsDir, 'cpbridge-extension.zip');
const outputZipLocal = path.join(extensionRoot, 'cpbridge-extension.zip');

if (!fs.existsSync(distDir) || !fs.existsSync(manifestFile)) {
  console.error('Error: dist directory or manifest.json not found. Run "vite build" first.');
  process.exit(1);
}

// Ensure target directories exist
fs.mkdirSync(webStaticDownloadsDir, { recursive: true });

async function createZip(outputPath) {
  return new Promise((resolve, reject) => {
    const output = fs.createWriteStream(outputPath);
    const archive = new ZipArchive({
      zlib: { level: 9 }
    });

    output.on('close', () => {
      console.log(`✓ Extension packaged successfully: ${outputPath} (${archive.pointer()} total bytes)`);
      resolve();
    });

    archive.on('error', (err) => {
      reject(err);
    });

    archive.pipe(output);

    // Append manifest.json
    archive.file(manifestFile, { name: 'manifest.json' });

    // Append all files in dist/ to dist/ folder in zip
    archive.directory(distDir, 'dist');

    archive.finalize();
  });
}

async function main() {
  console.log('Packaging cpbridge Chrome Extension...');
  try {
    await createZip(outputZipInWeb);
    // Also create local copy
    await createZip(outputZipLocal);
    console.log('Extension package complete!');
  } catch (err) {
    console.error('Failed to package extension:', err);
    process.exit(1);
  }
}

main();
