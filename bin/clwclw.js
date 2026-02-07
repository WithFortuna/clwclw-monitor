#!/usr/bin/env node

/**
 * clwclw — CLI entry point for clwclw-monitor agent.
 *
 * Dispatches to sub-commands:
 *   setup, install-hooks, version, heartbeat, hook, work, run
 */

const path = require('path');
const { ensureHome, loadDotEnvIfPresent, resolveHome } = require('../lib/config');

// Signal to lib/agent.js that we're running via the CLI binary
process.env.CLWCLW_CLI = '1';

// Set CLWCLW_VENDOR_DIR so agent.js finds vendored Claude-Code-Remote
if (!process.env.CLWCLW_VENDOR_DIR) {
  process.env.CLWCLW_VENDOR_DIR = path.join(__dirname, '..', 'vendor', 'claude-code-remote');
}

// Ensure home directory exists and load config
const home = ensureHome();
loadDotEnvIfPresent(path.join(home, '.env'));

// Set default AGENT_STATE_DIR to CLWCLW_HOME/data (unless already set)
if (!process.env.AGENT_STATE_DIR) {
  process.env.AGENT_STATE_DIR = path.join(home, 'data');
}

const cmd = process.argv[2] || '';
const args = process.argv.slice(3);

switch (cmd) {
  case 'version':
  case '--version':
  case '-v': {
    const pkg = require('../package.json');
    console.log(`${pkg.name} v${pkg.version}`);
    break;
  }

  case 'setup': {
    const { runSetup } = require('../lib/setup');
    runSetup().catch((err) => {
      console.error(String(err?.message || err));
      process.exit(1);
    });
    break;
  }

  case 'install-hooks': {
    const { installHooks } = require('../lib/hooks');
    installHooks().catch((err) => {
      console.error(String(err?.message || err));
      process.exit(1);
    });
    break;
  }

  case 'heartbeat':
  case 'hook':
  case 'work':
  case 'run': {
    // Delegate to agent.js with the full argument list
    const { main } = require('../lib/agent');
    main([cmd, ...args]).catch((err) => {
      console.error(String(err?.message || err));
      process.exit(1);
    });
    break;
  }

  case 'help':
  case '--help':
  case '-h':
  case '': {
    const pkg = require('../package.json');
    console.log(`${pkg.name} v${pkg.version}

Usage:
  clwclw setup              Interactive configuration wizard
  clwclw install-hooks      Install Claude Code hooks
  clwclw version            Show version

  clwclw heartbeat          Send heartbeat to Coordinator
  clwclw hook <type>        Run hook (completed|waiting)
  clwclw work [options]     Start task worker
    --channel <name>        Channel(s) to poll (comma-separated)
    --tmux-target <target>  tmux target for injection
  clwclw run                Start legacy services with heartbeat

Config:
  ~/.clwclw/.env            Configuration file (created by 'clwclw setup')
  ~/.clwclw/data/           Agent state directory

Env:
  COORDINATOR_URL           Coordinator server (default: http://localhost:8080)
  COORDINATOR_AUTH_TOKEN    API authentication token
  CLWCLW_HOME               Config directory (default: ~/.clwclw)
`);
    if (!cmd) process.exit(1);
    break;
  }

  default:
    console.error(`Unknown command: ${cmd}`);
    console.error('Run "clwclw help" for usage.');
    process.exit(1);
}
