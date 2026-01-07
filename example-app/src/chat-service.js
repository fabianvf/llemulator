
import OpenAI from 'openai';

export class ChatService {
  constructor(apiKey, baseURL) {
    this.openai = new OpenAI({
      apiKey: apiKey || process.env.OPENAI_API_KEY || 'dummy-key',
      baseURL: baseURL || process.env.OPENAI_BASE_URL || 'https://api.openai.com/v1',
    });
    
    this.conversationHistory = [];
    this.mode = 'helpful'; // helpful, creative, concise
    this.model = 'gpt-4';
  }
  
  setMode(mode) {
    const validModes = ['helpful', 'creative', 'concise'];
    if (!validModes.includes(mode)) {
      throw new Error(`Invalid mode: ${mode}. Must be one of: ${validModes.join(', ')}`);
    }
    this.mode = mode;
  }
  
  getSystemPrompt() {
    const prompts = {
      helpful: 'You are a helpful AI assistant. Provide clear, accurate, and useful responses.',
      creative: 'You are a creative AI assistant. Be imaginative, playful, and think outside the box.',
      concise: 'You are a concise AI assistant. Provide brief, direct answers without unnecessary elaboration.',
    };
    return prompts[this.mode];
  }
  
  clearHistory() {
    this.conversationHistory = [];
  }
  
  getHistory() {
    return [...this.conversationHistory];
  }
  
  async sendMessage(message, options = {}) {
    this.conversationHistory.push({
      role: 'user',
      content: message,
    });
    
    const messages = [
      { role: 'system', content: this.getSystemPrompt() },
      ...this.conversationHistory,
    ];
    
    try {
      const response = await this.openai.chat.completions.create({
        model: this.model,
        messages: messages,
        temperature: options.temperature || 0.7,
        max_tokens: options.maxTokens || 500,
        stream: options.stream || false,
      });
      
      if (options.stream) {
        return response;
      } else {
        const assistantMessage = response.choices[0].message;
        
        this.conversationHistory.push(assistantMessage);
        
        return assistantMessage.content;
      }
    } catch (error) {
      this.conversationHistory.pop();
      throw new Error(`Failed to get AI response: ${error.message}`);
    }
  }
  
  async streamMessage(message, onChunk) {
    this.conversationHistory.push({
      role: 'user',
      content: message,
    });
    
    const messages = [
      { role: 'system', content: this.getSystemPrompt() },
      ...this.conversationHistory,
    ];
    
    try {
      const stream = await this.openai.chat.completions.create({
        model: this.model,
        messages: messages,
        temperature: 0.7,
        max_tokens: 500,
        stream: true,
      });
      
      let fullContent = '';
      
      for await (const chunk of stream) {
        const content = chunk.choices[0]?.delta?.content || '';
        if (content) {
          fullContent += content;
          if (onChunk) {
            onChunk(content);
          }
        }
      }
      
      this.conversationHistory.push({
        role: 'assistant',
        content: fullContent,
      });
      
      return fullContent;
    } catch (error) {
      this.conversationHistory.pop();
      throw new Error(`Failed to stream AI response: ${error.message}`);
    }
  }
  
  async getSummary() {
    if (this.conversationHistory.length === 0) {
      return 'No conversation to summarize.';
    }
    
    const summaryPrompt = `Please provide a brief summary of the following conversation:\n\n${
      this.conversationHistory.map(msg => `${msg.role}: ${msg.content}`).join('\n')
    }\n\nSummary:`;
    
    try {
      const response = await this.openai.chat.completions.create({
        model: this.model,
        messages: [
          { role: 'system', content: 'You are a helpful assistant that creates concise summaries.' },
          { role: 'user', content: summaryPrompt },
        ],
        temperature: 0.5,
        max_tokens: 200,
      });
      
      return response.choices[0].message.content;
    } catch (error) {
      throw new Error(`Failed to generate summary: ${error.message}`);
    }
  }
}