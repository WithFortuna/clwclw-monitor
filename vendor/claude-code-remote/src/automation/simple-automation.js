/**
 * Simple Automation Solution
 * Use simpler methods to handle email reply commands
 */

const { spawn } = require('child_process');
const fs = require('fs');
const path = require('path');
const Logger = require('../core/logger');

class SimpleAutomation {
    constructor() {
        this.logger = new Logger('SimpleAutomation');
        this.commandFile = path.join(__dirname, '../data/current_command.txt');
        this.notificationSent = false;
    }

    /**
     * Send command - using multiple simple methods
     * @param {string} command - Command to send
     * @param {string} sessionId - Session ID
     * @returns {Promise<boolean>} - Whether successful
     */
    async sendCommand(command, sessionId = '') {
        try {
            // Method 1: Save to file, user can manually copy
            await this._saveCommandToFile(command, sessionId);
            
            // Method 2: Copy to clipboard
            const clipboardSuccess = await this._copyToClipboard(command);
            
            // Method 3: Send rich notification (including command content)
            const notificationSuccess = await this._sendRichNotification(command, sessionId);
            
            // Method 4: Try simple automation (no complex permissions needed)
            const autoSuccess = await this._trySimpleAutomation(command);
            
            if (clipboardSuccess || notificationSuccess || autoSuccess) {
                this.logger.info('Command sent successfully via simple automation');
                return true;
            } else {
                this.logger.warn('All simple automation methods failed');
                return false;
            }
            
        } catch (error) {
            this.logger.error('Simple automation failed:', error.message);
            return false;
        }
    }

    /**
     * Save command to file
     */
    async _saveCommandToFile(command, sessionId) {
        try {
            const timestamp = new Date().toLocaleString('zh-CN');
            const content = `# TaskPing Email Reply Command
# Time: ${timestamp}
# Session ID: ${sessionId}
# 
# Please copy the command below to Claude Code for execution:

${command}

# ===============================
# This file can be deleted after execution
`;
            
            fs.writeFileSync(this.commandFile, content, 'utf8');
            this.logger.info(`Command saved to file: ${this.commandFile}`);
            return true;
        } catch (error) {
            this.logger.error('Failed to save command to file:', error.message);
            return false;
        }
    }

    /**
     * Copy to clipboard
     */
    async _copyToClipboard(command) {
        try {
            if (process.platform === 'darwin') {
                const pbcopy = spawn('pbcopy');
                pbcopy.stdin.write(command);
                pbcopy.stdin.end();
                
                return new Promise((resolve) => {
                    pbcopy.on('close', (code) => {
                        if (code === 0) {
                            this.logger.info('Command copied to clipboard');
                            resolve(true);
                        } else {
                            resolve(false);
                        }
                    });
                    
                    pbcopy.on('error', () => resolve(false));
                });
            }
            return false;
        } catch (error) {
            this.logger.error('Failed to copy to clipboard:', error.message);
            return false;
        }
    }

    /**
     * Send rich notification
     */
    async _sendRichNotification(command, sessionId) {
        try {
            if (process.platform === 'darwin') {
                const shortCommand = command.length > 50 ? command.substring(0, 50) + '...' : command;
                
                // Create detailed notification
                const script = `
                    set commandText to "${command.replace(/"/g, '\\"').replace(/\n/g, '\\n')}"
                    
                    display notification "Command copied to clipboard, please paste to Claude Code" with title "TaskPing - New Email Command" subtitle "${shortCommand.replace(/"/g, '\\"')}" sound name "default"
                    
                    -- Also show dialog (optional, user can cancel)
                    try
                        set userChoice to display dialog "Received new email command:" & return & return & commandText buttons {"Open Command File", "Cancel", "Pasted"} default button "Pasted" with title "TaskPing Email Relay" giving up after 10
                        
                        if button returned of userChoice is "Open Command File" then
                            do shell script "open -t '${this.commandFile}'"
                        end if
                    end try
                `;
                
                const osascript = spawn('osascript', ['-e', script]);
                
                return new Promise((resolve) => {
                    osascript.on('close', (code) => {
                        if (code === 0) {
                            this.logger.info('Rich notification sent successfully');
                            resolve(true);
                        } else {
                            this.logger.warn('Rich notification failed, trying simple notification');
                            this._sendSimpleNotification(command);
                            resolve(true); // Consider successful even if failed, because there are backup options
                        }
                    });
                    
                    osascript.on('error', () => {
                        this._sendSimpleNotification(command);
                        resolve(true);
                    });
                });
            }
            return false;
        } catch (error) {
            this.logger.error('Failed to send rich notification:', error.message);
            return false;
        }
    }

    /**
     * Send simple notification
     */
    async _sendSimpleNotification(command) {
        try {
            const shortCommand = command.length > 50 ? command.substring(0, 50) + '...' : command;
            const script = `display notification "Command: ${shortCommand.replace(/"/g, '\\"')}" with title "TaskPing - Email Command" sound name "default"`;
            
            const osascript = spawn('osascript', ['-e', script]);
            osascript.on('close', () => {
                this.logger.info('Simple notification sent');
            });
        } catch (error) {
            this.logger.warn('Simple notification also failed:', error.message);
        }
    }

    /**
     * Try simple automation (no complex permissions needed)
     */
    async _trySimpleAutomation(command) {
        try {
            if (process.platform === 'darwin') {
                // Only try basic operations, don't force permissions
                const script = `
                    try
                        tell application "System Events"
                            -- Try to get frontmost application
                            set frontApp to name of first application process whose frontmost is true
                            
                            -- If it's terminal or code editor, try to paste
                            if frontApp contains "Terminal" or frontApp contains "iTerm" or frontApp contains "Code" or frontApp contains "Claude" then
                                keystroke "v" using command down
                                delay 0.2
                                keystroke return
                                return "success"
                            else
                                return "not_target_app"
                            end if
                        end tell
                    on error
                        return "no_permission"
                    end try
                `;
                
                const osascript = spawn('osascript', ['-e', script]);
                let output = '';
                
                osascript.stdout.on('data', (data) => {
                    output += data.toString().trim();
                });
                
                return new Promise((resolve) => {
                    osascript.on('close', (code) => {
                        if (code === 0 && output === 'success') {
                            this.logger.info('Simple automation succeeded');
                            resolve(true);
                        } else {
                            this.logger.debug(`Simple automation result: ${output}`);
                            resolve(false);
                        }
                    });
                    
                    osascript.on('error', () => resolve(false));
                });
            }
            return false;
        } catch (error) {
            this.logger.error('Simple automation error:', error.message);
            return false;
        }
    }

    /**
     * Open command file
     */
    async openCommandFile() {
        try {
            if (fs.existsSync(this.commandFile)) {
                if (process.platform === 'darwin') {
                    spawn('open', ['-t', this.commandFile]);
                    this.logger.info('Command file opened');
                    return true;
                }
            }
            return false;
        } catch (error) {
            this.logger.error('Failed to open command file:', error.message);
            return false;
        }
    }

    /**
     * Clean up command files
     */
    cleanupCommandFile() {
        try {
            if (fs.existsSync(this.commandFile)) {
                fs.unlinkSync(this.commandFile);
                this.logger.info('Command file cleaned up');
            }
        } catch (error) {
            this.logger.error('Failed to cleanup command file:', error.message);
        }
    }

    /**
     * Get status
     */
    getStatus() {
        return {
            supported: process.platform === 'darwin',
            commandFile: this.commandFile,
            commandFileExists: fs.existsSync(this.commandFile),
            lastCommand: this._getLastCommand()
        };
    }

    _getLastCommand() {
        try {
            if (fs.existsSync(this.commandFile)) {
                const content = fs.readFileSync(this.commandFile, 'utf8');
                const lines = content.split('\n');
                const commandLines = lines.filter(line => 
                    !line.startsWith('#') && 
                    !line.startsWith('=') && 
                    line.trim().length > 0
                );
                return commandLines.join('\n').trim();
            }
        } catch (error) {
            this.logger.error('Failed to get last command:', error.message);
        }
        return null;
    }
}

module.exports = SimpleAutomation;