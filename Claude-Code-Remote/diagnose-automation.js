#!/usr/bin/env node

/**
 * diagnose-automation.js
 *
 * Lightweight diagnostic helper used by:
 *   node claude-remote.js diagnose
 *
 * This file intentionally avoids extra dependencies so it can run in more environments.
 * It focuses on configuration, permissions prerequisites, and common failure points.
 */

const fs = require('fs');
const os = require('os');
const path = require('path');
const { spawnSync } = require('child_process');

class AutomationDiagnostic {
    async runDiagnostic() {
        this._printHeader();

        const projectRoot = __dirname;
        const envPath = path.join(projectRoot, '.env');
        const settingsPath = path.join(os.homedir(), '.claude', 'settings.json');

        const env = this._loadDotEnv(envPath);
        const injectionMode = (process.env.INJECTION_MODE || env.INJECTION_MODE || 'pty').trim();

        this._section('Environment');
        this._kv('OS', `${process.platform} (${os.release()})`);
        this._kv('Node', process.version);
        this._kv('CWD', process.cwd());
        this._kv('Project root', projectRoot);
        this._kv('.env', fs.existsSync(envPath) ? envPath : '(missing)');
        this._kv('INJECTION_MODE', injectionMode);

        this._section('Coordinator (optional)');
        this._kv('COORDINATOR_URL', process.env.COORDINATOR_URL || env.COORDINATOR_URL || '(not set)');
        this._kv('COORDINATOR_AUTH_TOKEN', (process.env.COORDINATOR_AUTH_TOKEN || env.COORDINATOR_AUTH_TOKEN) ? '(set)' : '(not set)');

        this._section('Hooks');
        this._kv('~/.claude/settings.json', fs.existsSync(settingsPath) ? settingsPath : '(missing)');
        this._diagnoseHooks(settingsPath);

        this._section('Channels (config completeness)');
        this._diagnoseChannels(env);

        this._section('Injection prerequisites');
        this._diagnoseInjection(injectionMode);

        this._section('Data/Logs');
        this._kv('Sessions dir', path.join(projectRoot, 'src', 'data', 'sessions'));
        this._kv('Session map', process.env.SESSION_MAP_PATH || env.SESSION_MAP_PATH || '(not set)');
        this._kv('Daemon PID', path.join(projectRoot, 'src', 'data', 'claude-code-remote.pid'));
        this._kv('Daemon log', path.join(projectRoot, 'src', 'data', 'daemon.log'));
        this._kv('Relay state', path.join(projectRoot, 'src', 'data', 'relay-state.json'));

        this._section('Next Actions');
        console.log('- Run interactive setup: `cd Claude-Code-Remote && npm run setup`');
        console.log('- Start services: `cd Claude-Code-Remote && npm run webhooks`');
        console.log('- Test notifications: `node Claude-Code-Remote/claude-hook-notify.js completed`');

        this._printFooter();
    }

    _printHeader() {
        console.log('╔══════════════════════════════════════════════════════════╗');
        console.log('║                 Claude-Code-Remote Diagnose              ║');
        console.log('╚══════════════════════════════════════════════════════════╝');
        console.log('');
    }

    _printFooter() {
        console.log('');
        console.log('✅ Diagnose completed');
    }

    _section(title) {
        console.log('');
        console.log(`== ${title} ==`);
    }

    _kv(key, value) {
        const pad = 22;
        const k = (key + ':').padEnd(pad, ' ');
        console.log(`${k}${value}`);
    }

    _warn(msg) {
        console.log(`⚠️  ${msg}`);
    }

    _ok(msg) {
        console.log(`✅ ${msg}`);
    }

    _loadDotEnv(envPath) {
        const out = {};
        if (!fs.existsSync(envPath)) return out;

        try {
            const content = fs.readFileSync(envPath, 'utf8');
            for (const rawLine of content.split(/\r?\n/)) {
                const line = rawLine.trim();
                if (!line || line.startsWith('#')) continue;
                const eq = line.indexOf('=');
                if (eq <= 0) continue;
                const key = line.slice(0, eq).trim();
                let val = line.slice(eq + 1).trim();
                if (!key) continue;

                if (
                    (val.startsWith('"') && val.endsWith('"')) ||
                    (val.startsWith("'") && val.endsWith("'"))
                ) {
                    val = val.slice(1, -1);
                }

                out[key] = val;
            }
        } catch (err) {
            this._warn(`Failed to read .env: ${err.message}`);
        }

        return out;
    }

    _diagnoseHooks(settingsPath) {
        if (!fs.existsSync(settingsPath)) {
            this._warn('Hooks file missing. Run `npm run setup` to install hooks.');
            return;
        }

        try {
            const settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
            const hooks = settings?.hooks || {};
            const stop = Array.isArray(hooks.Stop) ? hooks.Stop : [];
            const sub = Array.isArray(hooks.SubagentStop) ? hooks.SubagentStop : [];

            const commands = (entries) =>
                entries
                    .flatMap((e) => (Array.isArray(e?.hooks) ? e.hooks : []))
                    .map((h) => h?.command)
                    .filter(Boolean);

            const stopCmds = commands(stop);
            const subCmds = commands(sub);

            const hasLegacy = stopCmds.some((c) => String(c).includes('claude-hook-notify.js')) ||
                subCmds.some((c) => String(c).includes('claude-hook-notify.js'));

            const hasAgent = stopCmds.some((c) => String(c).includes('agent/clw-agent.js')) ||
                subCmds.some((c) => String(c).includes('agent/clw-agent.js'));

            if (hasLegacy && hasAgent) {
                this._warn('Both legacy and agent wrapper hooks detected. This may cause duplicate notifications.');
                this._warn('Re-run setup to reconcile: `cd Claude-Code-Remote && npm run setup`');
            } else if (hasAgent) {
                this._ok('Agent wrapper hooks detected (recommended).');
            } else if (hasLegacy) {
                this._ok('Legacy hooks detected (no Coordinator upload).');
            } else {
                this._warn('No matching hooks detected. Re-run setup to install hooks.');
            }

            this._kv('Stop hook commands', stopCmds.length ? stopCmds.join(' | ') : '(none)');
            this._kv('SubagentStop commands', subCmds.length ? subCmds.join(' | ') : '(none)');
        } catch (err) {
            this._warn(`Failed to parse hooks file: ${err.message}`);
        }
    }

    _diagnoseChannels(env) {
        const telegramEnabled = (process.env.TELEGRAM_ENABLED || env.TELEGRAM_ENABLED || '').toLowerCase() === 'true';
        const lineEnabled = (process.env.LINE_ENABLED || env.LINE_ENABLED || '').toLowerCase() === 'true';
        const emailEnabled = (process.env.EMAIL_ENABLED || env.EMAIL_ENABLED || '').toLowerCase() === 'true';

        this._kv('TELEGRAM_ENABLED', telegramEnabled ? 'true' : 'false');
        if (telegramEnabled) {
            const ok =
                !!(process.env.TELEGRAM_BOT_TOKEN || env.TELEGRAM_BOT_TOKEN) &&
                !!(process.env.TELEGRAM_CHAT_ID || env.TELEGRAM_CHAT_ID || process.env.TELEGRAM_GROUP_ID || env.TELEGRAM_GROUP_ID);
            ok ? this._ok('Telegram config looks complete') : this._warn('Telegram config incomplete (need BOT_TOKEN + CHAT_ID/GROUP_ID)');
        }

        this._kv('LINE_ENABLED', lineEnabled ? 'true' : 'false');
        if (lineEnabled) {
            const ok =
                !!(process.env.LINE_CHANNEL_ACCESS_TOKEN || env.LINE_CHANNEL_ACCESS_TOKEN) &&
                !!(process.env.LINE_CHANNEL_SECRET || env.LINE_CHANNEL_SECRET) &&
                !!(process.env.LINE_USER_ID || env.LINE_USER_ID || process.env.LINE_GROUP_ID || env.LINE_GROUP_ID);
            ok ? this._ok('LINE config looks complete') : this._warn('LINE config incomplete (need access token + secret + USER_ID/GROUP_ID)');
        }

        this._kv('EMAIL_ENABLED', emailEnabled ? 'true' : 'false');
        if (emailEnabled) {
            const ok =
                !!(process.env.SMTP_USER || env.SMTP_USER) &&
                !!(process.env.SMTP_PASS || env.SMTP_PASS) &&
                !!(process.env.IMAP_USER || env.IMAP_USER) &&
                !!(process.env.IMAP_PASS || env.IMAP_PASS) &&
                !!(process.env.EMAIL_TO || env.EMAIL_TO);
            ok ? this._ok('Email config looks complete') : this._warn('Email config incomplete (need SMTP/IMAP + EMAIL_TO)');
        }

        if (!telegramEnabled && !lineEnabled && !emailEnabled) {
            this._warn('No channels enabled. Enable at least one channel in .env.');
        }
    }

    _diagnoseInjection(injectionMode) {
        if (injectionMode === 'tmux') {
            const which = spawnSync('which', ['tmux'], { encoding: 'utf8' });
            if (which.status === 0 && (which.stdout || '').trim()) {
                this._ok(`tmux found: ${(which.stdout || '').trim()}`);
            } else {
                this._warn('tmux not found. Install it (e.g. `brew install tmux`) or switch to INJECTION_MODE=pty.');
            }
            return;
        }

        if (process.platform === 'darwin') {
            const osascript = spawnSync('which', ['osascript'], { encoding: 'utf8' });
            const pbcopy = spawnSync('which', ['pbcopy'], { encoding: 'utf8' });
            if (osascript.status === 0) this._ok('osascript available (macOS automation possible)');
            else this._warn('osascript not found (unexpected on macOS)');
            if (pbcopy.status === 0) this._ok('pbcopy available (clipboard injection possible)');
            else this._warn('pbcopy not found (unexpected on macOS)');

            this._kv('macOS permissions', 'Automation/Accessibility permissions may be required for auto-paste');
        } else {
            this._kv('Injection note', 'PTY/clipboard automation may vary by OS.');
        }
    }
}

module.exports = AutomationDiagnostic;

if (require.main === module) {
    const d = new AutomationDiagnostic();
    d.runDiagnostic().catch((err) => {
        // eslint-disable-next-line no-console
        console.error(err?.message || String(err));
        process.exit(1);
    });
}

