/**
 * Model Router - Per-model routing system
 * Routes different Claude models to different providers (like free-claude-code)
 */

/**
 * Default model routing configuration
 * Maps Claude model tiers to specific providers
 */
export const DEFAULT_MODEL_ROUTING = {
  // Opus-tier models -> highest quality provider
  opus: {
    models: ['claude-3-opus', 'claude-3.5-opus', 'gpt-4', 'gpt-4-turbo'],
    primaryProvider: 'glm',
    fallbackProviders: ['nvidia_nim', 'openrouter', 'zai']
  },
  
  // Sonnet-tier models -> balanced quality/speed
  sonnet: {
    models: ['claude-3-sonnet', 'claude-3.5-sonnet', 'gpt-4o', 'glm-5-turbo'],
    primaryProvider: 'glm',
    fallbackProviders: ['nvidia_nim', 'openrouter', 'minimax']
  },
  
  // Haiku-tier models -> fastest
  haiku: {
    models: ['claude-3-haiku', 'gpt-4o-mini', 'glm-4-flash', 'glm-4-air'],
    primaryProvider: 'minimax',
    fallbackProviders: ['glm', 'deepseek', 'ollama']
  }
};

/**
 * Model Router class
 */
export class ModelRouter {
  constructor(config) {
    this.config = config;
    this.routing = config.modelRouting || DEFAULT_MODEL_ROUTING;
  }
  
  /**
   * Detect model tier from model name
   */
  detectModelTier(model) {
    const modelLower = model.toLowerCase();
    
    for (const [tier, config] of Object.entries(this.routing)) {
      if (config.models.some(m => modelLower.includes(m.toLowerCase()))) {
        return tier;
      }
    }
    
    // Default to sonnet tier for unknown models
    return 'sonnet';
  }
  
  /**
   * Get routing configuration for a model tier
   */
  getRoutingForTier(tier) {
    return this.routing[tier] || this.routing.sonnet;
  }
  
  /**
   * Get primary provider for a model
   */
  getPrimaryProvider(model) {
    const tier = this.detectModelTier(model);
    const routing = this.getRoutingForTier(tier);
    return routing.primaryProvider;
  }
  
  /**
   * Get fallback providers for a model
   */
  getFallbackProviders(model) {
    const tier = this.detectModelTier(model);
    const routing = this.getRoutingForTier(tier);
    return routing.fallbackProviders || [];
  }
  
  /**
   * Get all providers for a model (primary + fallbacks)
   */
  getAllProvidersForModel(model) {
    const primary = this.getPrimaryProvider(model);
    const fallbacks = this.getFallbackProviders(model);
    return [primary, ...fallbacks];
  }
  
  /**
   * Check if routing is enabled
   */
  isRoutingEnabled() {
    return this.config.enableModelRouting !== false;
  }
  
  /**
   * Get routing priority order for a model
   */
  getProviderPriority(model, availableProviders) {
    if (!this.isRoutingEnabled()) {
      return availableProviders;
    }
    
    const allProviders = this.getAllProvidersForModel(model);
    
    // Filter to only available providers, maintaining priority order
    return allProviders.filter(p => availableProviders.includes(p));
  }
}

export default ModelRouter;
