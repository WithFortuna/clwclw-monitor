#!/usr/bin/env node

/**
 * vendor-ccr.js
 *
 * Copies Claude-Code-Remote source into vendor/claude-code-remote/
 * for npm packaging. Excludes runtime data, secrets, and git metadata.
 */

const fs = require('fs');
const path = require('path');

const SRC = path.resolve(__dirname, '..', 'Claude-Code-Remote');
const DEST = path.resolve(__dirname, '..', 'vendor', 'claude-code-remote');

const EXCLUDE = new Set([
  '.git',
  '.github',
  'node_modules',
  '.env',
  '.env.local',
  '.env.example',
  '.DS_Store',
  'package-lock.json',
]);

const EXCLUDE_DIRS = new Set([
  'src/data',
]);

function copyRecursive(src, dest, relPath) {
  const entries = fs.readdirSync(src, { withFileTypes: true });

  for (const entry of entries) {
    const name = entry.name;
    const srcPath = path.join(src, name);
    const destPath = path.join(dest, name);
    const rel = relPath ? `${relPath}/${name}` : name;

    if (EXCLUDE.has(name)) continue;
    if (EXCLUDE_DIRS.has(rel)) continue;

    if (entry.isDirectory()) {
      fs.mkdirSync(destPath, { recursive: true });
      copyRecursive(srcPath, destPath, rel);
    } else if (entry.isFile()) {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}

function main() {
  if (!fs.existsSync(SRC)) {
    console.error(`Source not found: ${SRC}`);
    console.error('Run: git submodule update --init');
    process.exit(1);
  }

  // Clean destination
  if (fs.existsSync(DEST)) {
    fs.rmSync(DEST, { recursive: true, force: true });
  }
  fs.mkdirSync(DEST, { recursive: true });

  copyRecursive(SRC, DEST, '');
  console.log(`Vendored Claude-Code-Remote → ${path.relative(process.cwd(), DEST)}`);

  // Patch claude-hook-notify.js to support DOTENV_PATH
  const hookNotifyPath = path.join(DEST, 'claude-hook-notify.js');
  if (fs.existsSync(hookNotifyPath)) {
    let content = fs.readFileSync(hookNotifyPath, 'utf8');
    content = content.replace(
      /const envPath = path\.join\(projectDir, '\.env'\);/,
      "const envPath = process.env.DOTENV_PATH || path.join(projectDir, '.env');"
    );
    fs.writeFileSync(hookNotifyPath, content, 'utf8');
    console.log('Patched claude-hook-notify.js for DOTENV_PATH support');
  }
}

main();
