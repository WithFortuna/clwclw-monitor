/**
 * config.js
 *
 * Shared configuration utilities for clwclw-monitor agent.
 */

const fs = require('fs');
const os = require('os');
const path = require('path');

/**
 * Resolve the clwclw home directory.
 * Priority: CLWCLW_HOME env → ~/.clwclw
 */
function resolveHome() {
  const env = (process.env.CLWCLW_HOME || '').trim();
  if (env) return path.resolve(env);
  return path.join(os.homedir(), '.clwclw');
}

/**
 * Ensure the home directory exists.
 */
function ensureHome() {
  const home = resolveHome();
  if (!fs.existsSync(home)) {
    fs.mkdirSync(home, { recursive: true });
  }
  return home;
}

/**
 * Best-effort .env loader (no npm dependencies).
 * Does not overwrite existing process.env values.
 */
function loadDotEnvIfPresent(filePath) {
  if (!filePath) return;
  if (!fs.existsSync(filePath)) return;

  try {
    const content = fs.readFileSync(filePath, 'utf8');
    for (const rawLine of content.split(/\r?\n/)) {
      const line = rawLine.trim();
      if (!line || line.startsWith('#')) continue;
      const eq = line.indexOf('=');
      if (eq <= 0) continue;
      const key = line.slice(0, eq).trim();
      let value = line.slice(eq + 1).trim();
      if (!key) continue;
      if (process.env[key] !== undefined && process.env[key] !== '') continue;

      // Remove wrapping quotes
      if (
        (value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))
      ) {
        value = value.slice(1, -1);
      }

      process.env[key] = value;
    }
  } catch {
    // ignore dotenv parsing errors (best-effort)
  }
}

/**
 * Resolve the vendor directory for Claude-Code-Remote.
 * Priority: CLWCLW_VENDOR_DIR env → <package>/vendor/claude-code-remote
 */
function resolveVendorDir() {
  const env = (process.env.CLWCLW_VENDOR_DIR || '').trim();
  if (env) return path.resolve(env);
  return path.join(__dirname, '..', 'vendor', 'claude-code-remote');
}

module.exports = {
  resolveHome,
  ensureHome,
  loadDotEnvIfPresent,
  resolveVendorDir,
};
