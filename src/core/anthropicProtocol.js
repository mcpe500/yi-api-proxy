/**
 * Anthropic Protocol Helpers
 * Shared utilities for handling Anthropic Messages API format
 */

/**
 * Normalize content blocks to standard format
 * Handles OpenAI and Anthropic content formats
 */
export function normalizeContent(content) {
  if (!content) return [];
  
  // If string, wrap in text block
  if (typeof content === 'string') {
    return [{ type: 'text', text: content }];
  }
  
  // If array, normalize each block
  if (Array.isArray(content)) {
    return content.map(block => normalizeContentBlock(block));
  }
  
  // If object, normalize it
  return [normalizeContentBlock(content)];
}

/**
 * Normalize a single content block
 */
export function normalizeContentBlock(block) {
  if (!block || typeof block !== 'object') {
    return { type: 'text', text: String(block || '') };
  }
  
  // Handle image_url (OpenAI format)
  if (block.image_url) {
    return {
      type: 'image',
      source: {
        type: 'base64',
        media_type: 'image/jpeg',
        data: extractBase64(block.image_url.url)
      }
    };
  }
  
  // Handle thinking blocks
  if (block.type === 'thinking') {
    return {
      type: 'thinking',
      thinking: block.thinking || block.text || ''
    };
  }
  
  // Handle tool_use blocks
  if (block.type === 'tool_use') {
    return {
      type: 'tool_use',
      id: block.id || generateId(),
      name: block.name,
      input: block.input || {}
    };
  }
  
  // Handle tool_result blocks
  if (block.type === 'tool_result') {
    return {
      type: 'tool_result',
      tool_use_id: block.tool_use_id || block.toolUseId,
      content: normalizeContent(block.content || '')
    };
  }
  
  // Default text block
  return {
    type: 'text',
    text: block.text || String(block)
  };
}

/**
 * Extract base64 data from URL
 */
function extractBase64(url) {
  if (!url) return '';
  
  if (url.startsWith('data:')) {
    // data:image/jpeg;base64,/9j/4AAQ...
    const match = url.match(/data:.*?;base64,(.*)/);
    return match ? match[1] : '';
  }
  
  return url;
}

/**
 * Generate a simple ID for tool_use blocks
 */
function generateId() {
  return 'toolu_' + Math.random().toString(36).substring(2, 12);
}

/**
 * Convert OpenAI messages format to Anthropic format
 */
export function openaiToAnthropic(messages) {
  return messages.map(msg => {
    const normalized = {
      role: msg.role,
      content: normalizeContent(msg.content)
    };
    
    // Handle name field
    if (msg.name) {
      normalized.name = msg.name;
    }
    
    return normalized;
  });
}

/**
 * Convert Anthropic messages format to OpenAI format
 */
export function anthropicToOpenai(messages) {
  return messages.map(msg => {
    const content = msg.content || '';
    
    // Handle array content
    if (Array.isArray(content)) {
      const openaiContent = content.map(block => {
        if (block.type === 'image' && block.source) {
          return {
            type: 'image_url',
            image_url: {
              url: `data:${block.source.media_type || 'image/jpeg'};base64,${block.source.data}`
            }
          };
        }
        if (block.type === 'tool_use') {
          return {
            type: 'tool_use',
            id: block.id,
            name: block.name,
            input: block.input
          };
        }
        if (block.type === 'tool_result') {
          return {
            type: 'tool_result',
            tool_use_id: block.tool_use_id,
            content: block.content
          };
        }
        // Skip thinking blocks in OpenAI format
        if (block.type === 'thinking') {
          return null;
        }
        return { type: 'text', text: block.text || '' };
      }).filter(Boolean);
      
      return { role: msg.role, content: openaiContent };
    }
    
    // String content
    return { role: msg.role, content };
  });
}

/**
 * Extract thinking blocks from content
 */
export function extractThinkingBlocks(content) {
  if (!Array.isArray(content)) return [];
  
  return content
    .filter(block => block.type === 'thinking')
    .map(block => block.thinking || block.text || '');
}

/**
 * Remove thinking blocks from content
 */
export function removeThinkingBlocks(content) {
  if (!Array.isArray(content)) return content;
  
  return content.filter(block => block.type !== 'thinking');
}

/**
 * Merge system messages into system field
 */
export function mergeSystemMessages(messages, system) {
  const systemMessages = messages.filter(m => m.role === 'system');
  const otherMessages = messages.filter(m => m.role !== 'system');
  
  let systemText = system || '';
  
  if (systemMessages.length > 0) {
    const extracted = systemMessages
      .map(m => {
        const content = m.content;
        if (typeof content === 'string') return content;
        if (Array.isArray(content)) {
          return content
            .filter(c => c.type === 'text')
            .map(c => c.text)
            .join('\n');
        }
        return '';
      })
      .join('\n');
    
    systemText = systemText ? systemText + '\n' + extracted : extracted;
  }
  
  return { system: systemText, messages: otherMessages };
}

/**
 * Detect if content contains tool calls
 */
export function hasToolCalls(content) {
  if (!Array.isArray(content)) return false;
  return content.some(block => block.type === 'tool_use');
}

/**
 * Detect if content contains tool results
 */
export function hasToolResults(content) {
  if (!Array.isArray(content)) return false;
  return content.some(block => block.type === 'tool_result');
}

/**
 * Validate Anthropic message format
 */
export function validateAnthropicMessage(message) {
  if (!message || typeof message !== 'object') {
    return { valid: false, error: 'Message must be an object' };
  }
  
  if (!['user', 'assistant', 'system'].includes(message.role)) {
    return { valid: false, error: `Invalid role: ${message.role}` };
  }
  
  if (message.content === undefined || message.content === null) {
    return { valid: false, error: 'Message must have content' };
  }
  
  return { valid: true };
}

export default {
  normalizeContent,
  normalizeContentBlock,
  openaiToAnthropic,
  anthropicToOpenai,
  extractThinkingBlocks,
  removeThinkingBlocks,
  mergeSystemMessages,
  hasToolCalls,
  hasToolResults,
  validateAnthropicMessage
};
