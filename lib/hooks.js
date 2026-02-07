/**
 * hooks.js
 *
 * Installs Claude Code hooks to ~/.claude/settings.json
 */

const fs = require('fs');
const os = require('os');
const path = require('path');

function claudeSettingsPath() {
  return path.join(os.homedir(), '.claude', 'settings.json');
}

function readSettings() {
  const file = claudeSettingsPath();
  if (!fs.existsSync(file)) {
    return { hooks: {} };
  }
  try {
    return JSON.parse(fs.readFileSync(file, 'utf8'));
  } catch {
    return { hooks: {} };
  }
}

function writeSettings(settings) {
  const file = claudeSettingsPath();
  const dir = path.dirname(file);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
  fs.writeFileSync(file, JSON.stringify(settings, null, 2) + '\n', 'utf8');
}

async function installHooks() {
  console.log('Installing Claude Code hooks...');

  const settings = readSettings();
  if (!settings.hooks) {
    settings.hooks = {};
  }

  const hookCmd = 'clwclw hook';

  // Stop hook
  const stopHook = `${hookCmd} completed`;
  if (!settings.hooks.Stop) {
    settings.hooks.Stop = stopHook;
    console.log(`  Added Stop hook: ${stopHook}`);
  } else if (settings.hooks.Stop !== stopHook) {
    console.log(`  Stop hook already exists: ${settings.hooks.Stop}`);
    console.log(`  (not overwriting; expected: ${stopHook})`);
  } else {
    console.log(`  Stop hook already configured.`);
  }

  // SubagentStop hook
  const subagentStopHook = `${hookCmd} waiting`;
  if (!settings.hooks.SubagentStop) {
    settings.hooks.SubagentStop = subagentStopHook;
    console.log(`  Added SubagentStop hook: ${subagentStopHook}`);
  } else if (settings.hooks.SubagentStop !== subagentStopHook) {
    console.log(`  SubagentStop hook already exists: ${settings.hooks.SubagentStop}`);
    console.log(`  (not overwriting; expected: ${subagentStopHook})`);
  } else {
    console.log(`  SubagentStop hook already configured.`);
  }

  writeSettings(settings);
  console.log(`\nHooks installed to ${claudeSettingsPath()}`);
}

module.exports = { installHooks };
