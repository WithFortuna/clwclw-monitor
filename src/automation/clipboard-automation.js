/**
 * Clipboard Automation
 * Send commands to Claude Code via clipboard and keyboard automation
 */

const { spawn } = require('child_process');
const Logger = require('../core/logger');

class ClipboardAutomation {
    constructor() {
        this.logger = new Logger('ClipboardAutomation');
    }

    /**
     * Send command to Claude Code (via clipboard)
     * @param {string} command - Command to send
     * @returns {Promise<boolean>} - Whether successful
     */
    async sendCommand(command) {
        try {
            // Step 1: Copy command to clipboard
            await this._copyToClipboard(command);
            
            // Step 2: Activate Claude Code and paste
            const success = await this._activateAndPaste();
            
            if (success) {
                this.logger.info('Command sent successfully via clipboard automation');
                return true;
            } else {
                this.logger.warn('Failed to activate Claude Code, trying fallback');
                // Try generic approach
                return await this._sendToActiveWindow(command);
            }
            
        } catch (error) {
            this.logger.error('Clipboard automation failed:', error.message);
            return false;
        }
    }

    /**
     * Copy text to clipboard
     */
    async _copyToClipboard(text) {
        return new Promise((resolve, reject) => {
            if (process.platform === 'darwin') {
                // macOS
                const pbcopy = spawn('pbcopy');
                pbcopy.stdin.write(text);
                pbcopy.stdin.end();
                
                pbcopy.on('close', (code) => {
                    if (code === 0) {
                        this.logger.debug('Text copied to clipboard');
                        resolve();
                    } else {
                        reject(new Error('Failed to copy to clipboard'));
                    }
                });
                
                pbcopy.on('error', reject);
            } else if (process.platform === 'linux') {
                // Linux (requires xclip or xsel)
                const xclip = spawn('xclip', ['-selection', 'clipboard']);
                xclip.stdin.write(text);
                xclip.stdin.end();
                
                xclip.on('close', (code) => {
                    if (code === 0) {
                        resolve();
                    } else {
                        // Try xsel
                        const xsel = spawn('xsel', ['--clipboard', '--input']);
                        xsel.stdin.write(text);
                        xsel.stdin.end();
                        
                        xsel.on('close', (code) => {
                            if (code === 0) {
                                resolve();
                            } else {
                                reject(new Error('Failed to copy to clipboard (xclip/xsel not available)'));
                            }
                        });
                    }
                });
            } else {
                reject(new Error('Clipboard automation not supported on this platform'));
            }
        });
    }

    /**
     * Activate Claude Code and paste command
     */
    async _activateAndPaste() {
        if (process.platform !== 'darwin') {
            return false;
        }

        return new Promise((resolve) => {
            const script = `
                tell application "System Events"
                    -- Try to find Claude Code related applications
                    set targetApps to {"Claude Code", "Terminal", "iTerm2", "iTerm", "Visual Studio Code", "Code", "Cursor"}
                    set foundApp to null
                    set appName to ""
                    
                    repeat with currentApp in targetApps
                        try
                            if application process currentApp exists then
                                set foundApp to application process currentApp
                                set appName to currentApp
                                exit repeat
                            end if
                        end try
                    end repeat
                    
                    if foundApp is not null then
                        -- Activate application
                        set frontmost of foundApp to true
                        delay 0.5
                        
                        -- Try to find input box and click
                        try
                            -- For some applications, may need to click specific input area
                            if appName is "Claude Code" then
                                -- Claude Code specific handling
                                key code 125 -- Down arrow, ensure cursor is in input box
                                delay 0.2
                            end if
                            
                            -- Paste content
                            keystroke "v" using command down
                            delay 0.3
                            
                            -- Send command (Enter)
                            keystroke return
                            
                            return "success"
                        on error errorMessage
                            return "paste_failed: " & errorMessage
                        end try
                    else
                        return "no_app_found"
                    end if
                end tell
            `;

            const osascript = spawn('osascript', ['-e', script]);
            let output = '';
            let error = '';

            osascript.stdout.on('data', (data) => {
                output += data.toString().trim();
            });

            osascript.stderr.on('data', (data) => {
                error += data.toString();
            });

            osascript.on('close', (code) => {
                if (code === 0 && output === 'success') {
                    this.logger.debug('Successfully activated app and pasted command');
                    resolve(true);
                } else {
                    this.logger.warn('AppleScript execution result:', { code, output, error });
                    resolve(false);
                }
            });

            osascript.on('error', (err) => {
                this.logger.error('AppleScript execution error:', err.message);
                resolve(false);
            });
        });
    }

    /**
     * Send to current active window (generic approach)
     */
    async _sendToActiveWindow(command) {
        if (process.platform !== 'darwin') {
            return false;
        }

        return new Promise((resolve) => {
            const script = `
                tell application "System Events"
                    -- Get current active application
                    set activeApp to name of first application process whose frontmost is true
                    
                    -- Paste command to current active window
                    keystroke "v" using command down
                    delay 0.3
                    keystroke return
                    
                    return "sent_to_" & activeApp
                end tell
            `;

            const osascript = spawn('osascript', ['-e', script]);
            let output = '';

            osascript.stdout.on('data', (data) => {
                output += data.toString().trim();
            });

            osascript.on('close', (code) => {
                if (code === 0) {
                    this.logger.debug('Command sent to active window:', output);
                    resolve(true);
                } else {
                    resolve(false);
                }
            });

            osascript.on('error', () => {
                resolve(false);
            });
        });
    }

    /**
     * Check if clipboard automation is supported
     */
    isSupported() {
        return process.platform === 'darwin' || process.platform === 'linux';
    }

    /**
     * Get current clipboard content (for testing)
     */
    async getClipboardContent() {
        if (process.platform === 'darwin') {
            return new Promise((resolve, reject) => {
                const pbpaste = spawn('pbpaste');
                let content = '';

                pbpaste.stdout.on('data', (data) => {
                    content += data.toString();
                });

                pbpaste.on('close', (code) => {
                    if (code === 0) {
                        resolve(content);
                    } else {
                        reject(new Error('Failed to read clipboard'));
                    }
                });
            });
        }
        return null;
    }
}

module.exports = ClipboardAutomation;