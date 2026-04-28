/**
 * Provider Catalog - Metadata for all supported providers
 * Based on free-claude-code provider registry
 */

export const PROVIDER_CATALOG = {
  // Existing providers
  glm: {
    name: 'GLM (Z.ai Coding Plan)',
    description: 'GLM models via Z.ai platform. Supports vision with GLM-5.1.',
    baseUrl: 'https://api.z.ai/api/coding/paas/v4',
    endpoint: '/chat/completions',
    type: 'openai',
    capabilities: ['vision', 'streaming', 'tools'],
    defaultModels: ['glm-5.1', 'glm-5-turbo', 'glm-4.7', 'glm-4-flash', 'glm-4-air'],
    visionModels: ['glm-5.1'],
    modelMap: {
      'claude-3-5-sonnet-20240620': 'glm-5-turbo',
      'claude-3-opus-20240229': 'glm-5.1',
      'claude-3-sonnet-20240229': 'glm-5-turbo',
      'gpt-4o': 'glm-5-turbo',
      'gpt-4o-mini': 'glm-4-flash'
    }
  },
  minimax: {
    name: 'MiniMax (Coding Plan)',
    description: 'MiniMax M2.7 via Anthropic-compatible endpoint. NO vision support.',
    baseUrl: 'https://api.minimax.io',
    endpoint: '/anthropic/v1/messages',
    type: 'anthropic',
    capabilities: ['text-only', 'streaming', 'tools'],
    defaultModels: ['MiniMax-M2.7'],
    visionModels: [],
    modelMap: {
      'minimax-m2.7': 'MiniMax-M2.7',
      'MiniMax-M2.7-highspeed': 'MiniMax-M2.7'
    }
  },
  zai: {
    name: 'Z.ai (Direct API)',
    description: 'Direct Z.ai API access for Claude models.',
    baseUrl: 'https://api.z.ai/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    capabilities: ['vision', 'streaming', 'tools'],
    defaultModels: ['glm-5.1', 'glm-5-turbo'],
    visionModels: ['glm-5.1', 'glm-5v-turbo']
  },

  // New providers (from free-claude-code)
  nvidia_nim: {
    name: 'NVIDIA NIM',
    description: 'NVIDIA NIM inference platform. Supports various models including GLM.',
    baseUrl: 'https://integrate.api.nvidia.com/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    capabilities: ['vision', 'streaming', 'tools'],
    defaultModels: ['nvidia_nim/z-ai/glm4.7', 'nvidia_nim/meta/llama-3.1-405b-instruct'],
    visionModels: ['nvidia_nim/z-ai/glm4.7'],
    modelMap: {
      'claude-3-5-sonnet': 'nvidia_nim/z-ai/glm4.7',
      'claude-3-opus': 'nvidia_nim/z-ai/glm4.7',
      'claude-3-sonnet': 'nvidia_nim/z-ai/glm4.7'
    }
  },
  openrouter: {
    name: 'OpenRouter',
    description: 'OpenRouter aggregates many AI models. Supports Claude, GPT, and more.',
    baseUrl: 'https://openrouter.ai/api/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    capabilities: ['vision', 'streaming', 'tools'],
    defaultModels: ['anthropic/claude-3.5-sonnet', 'anthropic/claude-3-opus', 'openai/gpt-4o'],
    visionModels: ['anthropic/claude-3.5-sonnet', 'anthropic/claude-3-opus', 'openai/gpt-4o'],
    modelMap: {
      'claude-3-5-sonnet': 'anthropic/claude-3.5-sonnet',
      'claude-3-opus': 'anthropic/claude-3-opus',
      'claude-3-sonnet': 'anthropic/claude-3-sonnet',
      'gpt-4o': 'openai/gpt-4o',
      'gpt-4o-mini': 'openai/gpt-4o-mini'
    }
  },
  deepseek: {
    name: 'DeepSeek',
    description: 'DeepSeek AI models. Cost-effective alternative.',
    baseUrl: 'https://api.deepseek.com/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    capabilities: ['text-only', 'streaming', 'tools'],
    defaultModels: ['deepseek-chat', 'deepseek-coder'],
    visionModels: [],
    modelMap: {
      'claude-3-sonnet': 'deepseek-chat',
      'gpt-4': 'deepseek-chat',
      'gpt-4-turbo': 'deepseek-chat'
    }
  },
  lm_studio: {
    name: 'LM Studio',
    description: 'Local LM Studio server. Run models locally.',
    baseUrl: 'http://localhost:1234/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    capabilities: ['vision', 'streaming', 'tools'],
    defaultModels: ['local-model'],
    visionModels: ['local-model'],
    modelMap: {}
  },
  llama_cpp: {
    name: 'llama.cpp',
    description: 'Local llama.cpp server. Run quantized models locally.',
    baseUrl: 'http://localhost:8080/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    capabilities: ['text-only', 'streaming'],
    defaultModels: ['local-model'],
    visionModels: [],
    modelMap: {}
  },
  ollama: {
    name: 'Ollama',
    description: 'Local Ollama server. Run open-source models locally.',
    baseUrl: 'http://localhost:11434/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    capabilities: ['vision', 'streaming', 'tools'],
    defaultModels: ['llama3', 'llama3:70b', 'mistral', 'codellama'],
    visionModels: ['llava', 'llava:13b'],
    modelMap: {
      'claude-3-sonnet': 'llama3:70b',
      'gpt-4': 'llama3:70b',
      'gpt-4-turbo': 'llama3:70b'
    }
  }
};

/**
 * Get provider metadata by name
 */
export function getProviderInfo(providerName) {
  return PROVIDER_CATALOG[providerName];
}

/**
 * Get all provider names
 */
export function getAllProviders() {
  return Object.keys(PROVIDER_CATALOG);
}

/**
 * Get providers by capability
 */
export function getProvidersByCapability(capability) {
  return Object.entries(PROVIDER_CATALOG)
    .filter(([_, info]) => info.capabilities.includes(capability))
    .map(([name, _]) => name);
}

/**
 * Get providers that support vision
 */
export function getVisionProviders() {
  return getProvidersByCapability('vision');
}

export default PROVIDER_CATALOG;
