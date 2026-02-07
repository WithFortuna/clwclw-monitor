/**
 * Conversation Tracker - Used to capture user questions and Claude responses
 */

const fs = require('fs');
const path = require('path');

class ConversationTracker {
    constructor() {
        this.conversationPath = path.join(__dirname, '../data/conversations.json');
        this.ensureDataDir();
    }
    
    ensureDataDir() {
        const dir = path.dirname(this.conversationPath);
        if (!fs.existsSync(dir)) {
            fs.mkdirSync(dir, { recursive: true });
        }
    }
    
    // Record user question
    recordUserMessage(sessionId, message) {
        const conversations = this.loadConversations();
        if (!conversations[sessionId]) {
            conversations[sessionId] = {
                created: new Date().toISOString(),
                messages: []
            };
        }
        
        conversations[sessionId].messages.push({
            type: 'user',
            content: message,
            timestamp: new Date().toISOString()
        });
        
        this.saveConversations(conversations);
    }
    
    // Record Claude response
    recordClaudeResponse(sessionId, response) {
        const conversations = this.loadConversations();
        if (!conversations[sessionId]) {
            conversations[sessionId] = {
                created: new Date().toISOString(),
                messages: []
            };
        }
        
        conversations[sessionId].messages.push({
            type: 'claude',
            content: response,
            timestamp: new Date().toISOString()
        });
        
        this.saveConversations(conversations);
    }
    
    // Get recent conversation content
    getRecentConversation(sessionId, limit = 2) {
        const conversations = this.loadConversations();
        const session = conversations[sessionId];
        
        if (!session || !session.messages.length) {
            return { userQuestion: '', claudeResponse: '' };
        }
        
        const messages = session.messages.slice(-limit * 2); // Get recent user-Claude conversation
        let userQuestion = '';
        let claudeResponse = '';
        
        // Find most recent user question and Claude response from back to front
        for (let i = messages.length - 1; i >= 0; i--) {
            const msg = messages[i];
            if (msg.type === 'claude' && !claudeResponse) {
                claudeResponse = msg.content;
            } else if (msg.type === 'user' && !userQuestion) {
                userQuestion = msg.content;
            }
            
            if (userQuestion && claudeResponse) break;
        }
        
        return {
            userQuestion: userQuestion || 'Unrecorded user question',
            claudeResponse: claudeResponse || 'Unrecorded Claude response'
        };
    }
    
    // Clean up expired conversations (older than 7 days)
    cleanupOldConversations() {
        const conversations = this.loadConversations();
        const now = new Date();
        const sevenDaysAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
        
        const cleaned = {};
        for (const [sessionId, session] of Object.entries(conversations)) {
            const created = new Date(session.created);
            if (created > sevenDaysAgo) {
                cleaned[sessionId] = session;
            }
        }
        
        this.saveConversations(cleaned);
        return Object.keys(conversations).length - Object.keys(cleaned).length;
    }
    
    loadConversations() {
        if (!fs.existsSync(this.conversationPath)) {
            return {};
        }
        
        try {
            return JSON.parse(fs.readFileSync(this.conversationPath, 'utf8'));
        } catch (error) {
            console.error('Failed to load conversations:', error);
            return {};
        }
    }
    
    saveConversations(conversations) {
        try {
            fs.writeFileSync(this.conversationPath, JSON.stringify(conversations, null, 2));
        } catch (error) {
            console.error('Failed to save conversations:', error);
        }
    }
}

module.exports = ConversationTracker;