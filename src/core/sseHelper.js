/**
 * SSE (Server-Sent Events) Helper
 * Handles streaming responses for both OpenAI and Anthropic formats
 */

/**
 * Parse SSE stream line
 */
export function parseSSELine(line) {
  if (!line || line.trim() === '') return null;
  
  // Remove "data: " prefix
  if (line.startsWith('data: ')) {
    const data = line.slice(6);
    
    // Handle [DONE] marker
    if (data === '[DONE]') {
      return { type: 'done' };
    }
    
    try {
      return { type: 'data', data: JSON.parse(data) };
    } catch (e) {
      return { type: 'error', error: e.message };
    }
  }
  
  return null;
}

/**
 * Convert Anthropic SSE event to OpenAI format
 */
export function anthropicToOpenaiChunk(anthropicEvent) {
  if (!anthropicEvent) return null;
  
  const chunk = {
    id: anthropicEvent.message_id || 'chatcmpl-' + generateId(),
    object: 'chat.completion.chunk',
    created: Date.now(),
    model: anthropicEvent.model || 'unknown',
    choices: []
  };
  
  if (anthropicEvent.type === 'content_block_delta') {
    chunk.choices.push({
      index: 0,
      delta: {
        content: anthropicEvent.delta?.text || ''
      },
      finish_reason: null
    });
  } else if (anthropicEvent.type === 'message_delta') {
    chunk.choices.push({
      index: 0,
      delta: {},
      finish_reason: anthropicEvent.delta?.stop_reason || null
    });
  } else if (anthropicEvent.type === 'message_start') {
    chunk.choices.push({
      index: 0,
      delta: {
        role: 'assistant'
      },
      finish_reason: null
    });
  }
  
  return chunk;
}

/**
 * Convert OpenAI SSE event to Anthropic format
 */
export function openaiToAnthropicChunk(openaiEvent) {
  if (!openaiEvent) return null;
  
  const delta = openaiEvent.choices?.[0]?.delta;
  const finishReason = openaiEvent.choices?.[0]?.finish_reason;
  
  if (delta?.content) {
    return {
      type: 'content_block_delta',
      index: 0,
      delta: { type: 'text_delta', text: delta.content }
    };
  }
  
  if (finishReason) {
    return {
      type: 'message_delta',
      delta: { stop_reason: finishReason, stop_sequence: null },
      usage: { output_tokens: openaiEvent.usage?.completion_tokens || 0 }
    };
  }
  
  if (delta?.role) {
    return {
      type: 'message_start',
      message: {
        id: openaiEvent.id,
        type: 'message',
        role: 'assistant',
        content: [],
        model: openaiEvent.model
      }
    };
  }
  
  return null;
}

/**
 * Generate a simple ID
 */
function generateId() {
  return Math.random().toString(36).substring(2, 15);
}

/**
 * Stream transformer for converting between formats
 */
export class StreamTransformer {
  constructor(targetFormat = 'openai') {
    this.targetFormat = targetFormat;
  }
  
  transform(chunk) {
    if (this.targetFormat === 'openai') {
      return anthropicToOpenaiChunk(chunk);
    } else if (this.targetFormat === 'anthropic') {
      return openaiToAnthropicChunk(chunk);
    }
    return chunk;
  }
}

/**
 * Process SSE stream line by line
 */
export async function* processSSEStream(stream, targetFormat = 'openai') {
  const transformer = new StreamTransformer(targetFormat);
  const decoder = new TextDecoder();
  let buffer = '';
  
  for await (const chunk of stream) {
    buffer += decoder.decode(chunk, { stream: true });
    
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';
    
    for (const line of lines) {
      const parsed = parseSSELine(line);
      
      if (parsed?.type === 'data') {
        const transformed = transformer.transform(parsed.data);
        if (transformed) {
          yield transformed;
        }
      } else if (parsed?.type === 'done') {
        return;
      }
    }
  }
  
  // Process remaining buffer
  if (buffer.trim()) {
    const parsed = parseSSELine(buffer);
    if (parsed?.type === 'data') {
      const transformed = transformer.transform(parsed.data);
      if (transformed) {
        yield transformed;
      }
    }
  }
}

export default {
  parseSSELine,
  anthropicToOpenaiChunk,
  openaiToAnthropicChunk,
  StreamTransformer,
  processSSEStream
};
