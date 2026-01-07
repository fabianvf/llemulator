#!/usr/bin/env node

/**
 * CLI Interface for the chat application
 */

import readline from 'readline';
import { ChatService } from './chat-service.js';
import chalk from 'chalk';

const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  prompt: chalk.blue('You> '),
});

// Initialize chat service
const chatService = new ChatService(
  process.env.OPENAI_API_KEY,
  process.env.OPENAI_BASE_URL
);

// Display welcome message
console.log(chalk.green('🤖 AI Chat Application'));
console.log(chalk.gray('Commands: /mode [helpful|creative|concise], /clear, /history, /summary, /exit'));
console.log(chalk.gray(`Current mode: ${chatService.mode}\n`));

rl.prompt();

rl.on('line', async (input) => {
  const line = input.trim();
  
  // Handle commands
  if (line.startsWith('/')) {
    const [command, ...args] = line.split(' ');
    
    switch (command) {
      case '/mode':
        const mode = args[0];
        if (!mode) {
          console.log(chalk.yellow(`Current mode: ${chatService.mode}`));
        } else {
          try {
            chatService.setMode(mode);
            console.log(chalk.green(`Mode changed to: ${mode}`));
          } catch (error) {
            console.log(chalk.red(error.message));
          }
        }
        break;
        
      case '/clear':
        chatService.clearHistory();
        console.log(chalk.green('Conversation history cleared.'));
        break;
        
      case '/history':
        const history = chatService.getHistory();
        if (history.length === 0) {
          console.log(chalk.gray('No conversation history.'));
        } else {
          console.log(chalk.cyan('\n--- Conversation History ---'));
          history.forEach((msg) => {
            const prefix = msg.role === 'user' ? 'You:' : 'AI:';
            const color = msg.role === 'user' ? chalk.blue : chalk.green;
            console.log(color(`${prefix} ${msg.content}`));
          });
          console.log(chalk.cyan('--- End History ---\n'));
        }
        break;
        
      case '/summary':
        console.log(chalk.gray('Generating summary...'));
        try {
          const summary = await chatService.getSummary();
          console.log(chalk.cyan('\nSummary:'), summary, '\n');
        } catch (error) {
          console.log(chalk.red(`Error: ${error.message}`));
        }
        break;
        
      case '/exit':
        console.log(chalk.green('Goodbye! 👋'));
        process.exit(0);
        break;
        
      default:
        console.log(chalk.red(`Unknown command: ${command}`));
        break;
    }
  } else if (line) {
    // Send message to AI
    process.stdout.write(chalk.green('AI> '));
    
    try {
      // Use streaming for a more interactive experience
      await chatService.streamMessage(line, (chunk) => {
        process.stdout.write(chalk.green(chunk));
      });
      console.log(); // New line after response
    } catch (error) {
      console.log(chalk.red(`\nError: ${error.message}`));
    }
  }
  
  rl.prompt();
});

rl.on('close', () => {
  console.log(chalk.green('\nGoodbye! 👋'));
  process.exit(0);
});