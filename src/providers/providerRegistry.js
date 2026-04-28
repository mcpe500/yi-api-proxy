/**
 * Provider Registry
 * Manages provider instances and routing
 */

import { PROVIDER_CATALOG, getProviderInfo } from '../config/providerCatalog.js';

/**
 * Provider class for handling API requests
 */
export class Provider {
  constructor(name, config) {
    this.name = name;
    this.config = config;
    this.info = getProviderInfo(name) || {};
  }
  
  /**
   * Get the base URL for this provider
   */
  get baseUrl() {
    return this.config.baseUrl || this.info.baseUrl;
  }
  
  /**
   * Get the endpoint for this provider
   */
  get endpoint() {
    return this.config.endpoint || this.info.endpoint;
  }
  
  /**
   * Get the API key for this provider
   */
  get apiKey() {
    return this.config.apiKey;
  }
  
  /**
   * Get the provider type (openai or anthropic)
   */
  get type() {
    return this.config.type || this.info.type || 'openai';
  }
  
  /**
   * Get capabilities
   */
  get capabilities() {
    return this.config.capabilities || this.info.capabilities || [];
  }
  
  /**
   * Check if provider supports vision
   */
  supportsVision(model = null) {
    if (model && this.config.visionModels?.includes(model)) {
      return true;
    }
    return this.capabilities.includes('vision');
  }
  
  /**
   * Check if provider supports streaming
   */
  supportsStreaming() {
    return this.capabilities.includes('streaming');
  }
  
  /**
   * Check if provider supports tools
   */
  supportsTools() {
    return this.capabilities.includes('tools');
  }
  
  /**
   * Map a model name to provider-specific model
   */
  mapModel(model) {
    // Check provider-specific model map
    if (this.config.modelMap?.[model]) {
      return this.config.modelMap[model];
    }
    
    // Check catalog model map
    if (this.info.modelMap?.[model]) {
      return this.info.modelMap[model];
    }
    
    // Check case-insensitive
    const modelLower = model.toLowerCase();
    if (this.config.modelMap?.[modelLower]) {
      return this.config.modelMap[modelLower];
    }
    if (this.info.modelMap?.[modelLower]) {
      return this.info.modelMap[modelLower];
    }
    
    // Return original if no mapping found
    return model;
  }
  
  /**
   * Build headers for API request
   */
  buildHeaders() {
    const headers = {
      'Content-Type': 'application/json'
    };
    
    if (this.type === 'anthropic') {
      headers['x-api-key'] = this.apiKey;
      headers['anthropic-version'] = '2023-06-01';
    } else {
      headers['Authorization'] = `Bearer ${this.apiKey}`;
    }
    
    // Add extra headers if configured
    if (this.config.extraHeaders) {
      Object.assign(headers, this.config.extraHeaders);
    }
    
    return headers;
  }
  
  /**
   * Build request body for this provider
   */
  buildBody(body, model) {
    const mappedModel = this.mapModel(model);
    const providerBody = {
      model: mappedModel,
      ...body
    };
    
    // Provider-specific adjustments
    if (this.type === 'anthropic') {
      // Anthropic Messages API already uses "messages" field — pass through as-is
    }
    
    return providerBody;
  }
}

/**
 * Provider Registry class
 */
export class ProviderRegistry {
  constructor(config) {
    this.config = config;
    this.providers = new Map();
    this.initializeProviders();
  }
  
  /**
   * Initialize all configured providers
   */
  initializeProviders() {
    for (const [name, providerConfig] of Object.entries(this.config.providers || {})) {
      if (providerConfig.apiKey && !providerConfig.apiKey.startsWith('YOUR_')) {
        this.providers.set(name, new Provider(name, providerConfig));
      }
    }
  }
  
  /**
   * Get a provider by name
   */
  getProvider(name) {
    return this.providers.get(name);
  }
  
  /**
   * Get all available providers
   */
  getAllProviders() {
    return Array.from(this.providers.keys());
  }
  
  /**
   * Get providers that support a specific capability
   */
  getProvidersByCapability(capability) {
    return Array.from(this.providers.values())
      .filter(p => p.capabilities.includes(capability))
      .map(p => p.name);
  }
  
  /**
   * Get vision-capable providers
   */
  getVisionProviders() {
    return this.getProvidersByCapability('vision');
  }
  
  /**
   * Resolve provider for a model
   */
  resolveProviderForModel(model) {
    const modelLower = model.toLowerCase();
    
    // Check aliases
    for (const [alias, providerName] of Object.entries(this.config.aliases || {})) {
      if (modelLower.includes(alias)) {
        const provider = this.getProvider(providerName);
        if (provider) return provider;
      }
    }
    
    // Check provider model lists
    for (const provider of this.providers.values()) {
      const models = [...(provider.config.models || []), ...(provider.info.defaultModels || [])];
      if (models.some(m => modelLower.startsWith(m.toLowerCase()))) {
        return provider;
      }
    }
    
    // Default to first available provider
    const firstProvider = this.providers.values().next().value;
    return firstProvider;
  }
  
  /**
   * Check if registry has any providers
   */
  hasProviders() {
    return this.providers.size > 0;
  }
}

export default ProviderRegistry;
