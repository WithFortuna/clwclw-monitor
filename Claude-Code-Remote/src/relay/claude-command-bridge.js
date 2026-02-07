/**
 * Claude Command Bridge
 * Bridge for communicating with Claude Code via file system
 */

const fs = require('fs');
const path = require('path');
const Logger = require('../core/logger');

class ClaudeCommandBridge {
    constructor() {
        this.logger = new Logger('CommandBridge');
        this.commandsDir = path.join(__dirname, '../data/commands');
        this.responseDir = path.join(__dirname, '../data/responses');
        
        this._ensureDirectories();
    }

    _ensureDirectories() {
        [this.commandsDir, this.responseDir].forEach(dir => {
            if (!fs.existsSync(dir)) {
                fs.mkdirSync(dir, { recursive: true });
            }
        });
    }

    /**
     * Send command to Claude Code
     * @param {string} command - Command to execute
     * @param {string} sessionId - Session ID
     * @returns {Promise<boolean>} - Whether successful
     */
    async sendCommand(command, sessionId) {
        try {
            const timestamp = Date.now();
            const commandId = `${sessionId}_${timestamp}`;
            
            // Create command file
            const commandFile = path.join(this.commandsDir, `${commandId}.json`);
            const commandData = {
                id: commandId,
                sessionId,
                command,
                timestamp: new Date().toISOString(),
                status: 'pending',
                source: 'email'
            };
            
            fs.writeFileSync(commandFile, JSON.stringify(commandData, null, 2));
            
            // Create notification file (Claude Code can monitor this file for changes)
            const notificationFile = path.join(this.commandsDir, '.new_command');
            fs.writeFileSync(notificationFile, commandId);
            
            this.logger.info(`Command sent via file bridge: ${commandId}`);
            
            // Try to use system notification
            await this._sendSystemNotification(command, commandId);
            
            return true;
            
        } catch (error) {
            this.logger.error('Failed to send command via bridge:', error.message);
            return false;
        }
    }

    async _sendSystemNotification(command, commandId) {
        try {
            const title = 'TaskPing - New Email Command';
            const body = `Command: ${command.length > 50 ? command.substring(0, 50) + '...' : command}\n\nClick to view details or enter command in Claude Code`;
            
            if (process.platform === 'darwin') {
                // macOS notification
                const script = `
                    display notification "${body.replace(/"/g, '\\"')}" with title "${title}" sound name "default"
                `;
                const { spawn } = require('child_process');
                spawn('osascript', ['-e', script]);
            } else if (process.platform === 'linux') {
                // Linux notification
                const { spawn } = require('child_process');
                spawn('notify-send', [title, body]);
            }
            
            this.logger.debug('System notification sent');
        } catch (error) {
            this.logger.warn('Failed to send system notification:', error.message);
        }
    }

    /**
     * Get pending commands
     * @returns {Array} Command list
     */
    getPendingCommands() {
        try {
            const files = fs.readdirSync(this.commandsDir)
                .filter(file => file.endsWith('.json'))
                .sort();
            
            const commands = [];
            for (const file of files) {
                try {
                    const commandData = JSON.parse(
                        fs.readFileSync(path.join(this.commandsDir, file), 'utf8')
                    );
                    if (commandData.status === 'pending') {
                        commands.push(commandData);
                    }
                } catch (error) {
                    this.logger.warn(`Failed to parse command file ${file}:`, error.message);
                }
            }
            
            return commands;
        } catch (error) {
            this.logger.error('Failed to get pending commands:', error.message);
            return [];
        }
    }

    /**
     * Mark command as processed
     * @param {string} commandId - Command ID
     * @param {string} status - Status (completed/failed)
     * @param {string} response - Response content
     */
    markCommandProcessed(commandId, status = 'completed', response = '') {
        try {
            const commandFile = path.join(this.commandsDir, `${commandId}.json`);
            
            if (fs.existsSync(commandFile)) {
                const commandData = JSON.parse(fs.readFileSync(commandFile, 'utf8'));
                commandData.status = status;
                commandData.processedAt = new Date().toISOString();
                commandData.response = response;
                
                // Save to response directory
                const responseFile = path.join(this.responseDir, `${commandId}.json`);
                fs.writeFileSync(responseFile, JSON.stringify(commandData, null, 2));
                
                // Delete original command file
                fs.unlinkSync(commandFile);
                
                this.logger.info(`Command ${commandId} marked as ${status}`);
            }
        } catch (error) {
            this.logger.error(`Failed to mark command ${commandId} as processed:`, error.message);
        }
    }

    /**
     * Clean up old command and response files
     * @param {number} maxAge - Maximum retention time (hours)
     */
    cleanup(maxAge = 24) {
        const cutoff = Date.now() - (maxAge * 60 * 60 * 1000);
        let cleaned = 0;
        
        [this.commandsDir, this.responseDir].forEach(dir => {
            try {
                const files = fs.readdirSync(dir).filter(file => file.endsWith('.json'));
                
                for (const file of files) {
                    const filePath = path.join(dir, file);
                    const stats = fs.statSync(filePath);
                    
                    if (stats.mtime.getTime() < cutoff) {
                        fs.unlinkSync(filePath);
                        cleaned++;
                    }
                }
            } catch (error) {
                this.logger.warn(`Failed to cleanup directory ${dir}:`, error.message);
            }
        });
        
        if (cleaned > 0) {
            this.logger.info(`Cleaned up ${cleaned} old command files`);
        }
    }

    /**
     * Get bridge status
     */
    getStatus() {
        try {
            const pendingCommands = this.getPendingCommands();
            const responseFiles = fs.readdirSync(this.responseDir).filter(f => f.endsWith('.json'));
            
            return {
                pendingCommands: pendingCommands.length,
                processedCommands: responseFiles.length,
                commandsDir: this.commandsDir,
                responseDir: this.responseDir,
                recentCommands: pendingCommands.slice(-5).map(cmd => ({
                    id: cmd.id,
                    command: cmd.command.substring(0, 50) + '...',
                    timestamp: cmd.timestamp
                }))
            };
        } catch (error) {
            this.logger.error('Failed to get bridge status:', error.message);
            return {
                pendingCommands: 0,
                processedCommands: 0,
                error: error.message
            };
        }
    }
}

module.exports = ClaudeCommandBridge;