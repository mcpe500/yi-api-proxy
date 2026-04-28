/**
 * Chat Routes - API endpoints for chat completions
 * Similar to free-claude-code's API layer
 */

import express from 'express';
import { ProviderRegistry } from '../providers/providerRegistry.js';
import { ModelRouter } from '../config/modelRouter.js';
import { getProviderInfo } from '../config/providerCatalog.js';
import { openaiToAnthropic, anthropicToOpenai, mergeSystemMessages, removeThinkingBlocks } from '../core/anthropicProtocol.js';
import { processSSEStream, anthropicToOpenaiChunk } from '../core/sseHelper.js';
import { hasImages, processImages } from '../middleware/mediaHandler.js';
import { log, logProxy, logError } from '../lib/logger.js';

const router = express.Router();

// Retryable error types
const RETRYABLE_ERRORS = ['RATE_LIMITED', 'SERVER_ERROR', 'TIMEOUT', 'INSUFFICIENT_BALANCE', 'NETWORK_ERROR'];

/**
 * POST /v1/chat/completions
 * Universal chat completion endpoint
 */
router.post('/chat/completions', handleChatCompletions);

/**
 * POST /v1/messages
 * Anthropic Messages API endpoint
 */
router.post('/messages', handleChatCompletions);

/**
 * GET /v1/models
 * List available models
 */
router.get('/models', handleListModels);

/**
 * GET /v1/providers
 * List all providers
 */
router.get('/providers', handleListProviders);

/**
 * GET /v1/health/:provider
 * Health check for specific provider
 */
router.get('/health/:provider', handleProviderHealth);

/**
 * Main chat completion handler
 */
async function handleChatCompletions(req, res) {
  const requestId = req.requestId || 'unknown';
  const config = global.config;
  
  // Initialize provider registry and model router
  const registry = new ProviderRegistry(config);
  const modelRouter = new ModelRouter(config);
  
  if (!registry.hasProviders()) {
    return res.status(500).json({
      error: 'No providers configured',
      message: 'Add API keys to config.json'
    });
  }
  
  const { provider: requestedProvider, model: requestedModel } = req.headers;
  const targetModel = requestedModel || req.body?.model;
  
  logProxy(requestId, requestedProvider || 'auto', targetModel, 'Request received');
  
  if (!targetModel) {
    return res.status(400).json({
      error: 'Missing model',
      message: 'Provide model in body or X-Model header',
      requestId
    });
  }
  
  // Determine provider
  let targetProvider = requestedProvider?.toLowerCase();
  
  if (!targetProvider && modelRouter.isRoutingEnabled()) {
    // Use model routing to determine provider
    targetProvider = modelRouter.getPrimaryProvider(targetModel);
  }
  
  if (!targetProvider) {
    // Auto-detect from model name
    const provider = registry.resolveProviderForModel(targetModel);
    if (provider) {
      targetProvider = provider.name;
    }
  }
  
  const provider = registry.getProvider(targetProvider);
  
  if (!provider) {
    return res.status(400).json({
      error: 'Provider not found or not configured',
      requestedProvider: targetProvider,
      available: registry.getAllProviders(),
      hint: 'Configure provider in config.json'
    });
  }
  
  // Handle images
  let processedBody = req.body;
  let mediaProcessed = false;
  let visionProvider = null;
  
  if (hasImages(req.body)) {
    const hasVision = provider.supportsVision(targetModel);
    
    if (!hasVision && config.visionFallback?.enabled) {
      try {
        const { modifiedBody, visionProv } = await processImages(req.body, config, targetModel);
        if (modifiedBody) {
          processedBody = modifiedBody;
          mediaProcessed = true;
          visionProvider = visionProv;
        }
      } catch (error) {
        logError(requestId, error, { provider: targetProvider, stage: 'vision' });
      }
    } else if (!hasVision) {
      return res.status(400).json({
        error: 'Provider does not support vision',
        provider: targetProvider,
        model: targetModel,
        hint: 'Use a vision-capable provider or enable vision fallback'
      });
    }
  }
  
  // Build request
  const headers = provider.buildHeaders();
  const body = provider.buildBody(processedBody, targetModel);
  const url = provider.baseUrl + provider.endpoint;
  
  logProxy(requestId, targetProvider, targetModel, 'Forwarding request', {
    url: url.replace(provider.apiKey, '***'),
    hasImages: hasImages(req.body),
    mediaProcessed
  });
  
  // Try primary provider with fallback
  const result = await tryProviderWithFallback(
    url,
    headers,
    body,
    provider,
    targetModel,
    registry,
    modelRouter,
    requestId,
    mediaProcessed,
    visionProvider
  );
  
  if (result.error) {
    return res.status(result.status || 502).json({
      error: result.error,
      message: result.message,
      provider: result.provider,
      requestId
    });
  }
  
  // Send response
  await sendResponse(res, result.response, provider.name, targetModel, mediaProcessed, visionProvider, requestId);
}

/**
 * Try provider with fallback support
 */
async function tryProviderWithFallback(url, headers, body, provider, model, registry, modelRouter, requestId, mediaProcessed, visionProvider, attemptedProviders = []) {
  attemptedProviders.push(provider.name);
  
  const { response, error } = await forwardRequest(url, headers, body, provider.config.timeout || 3600000);
  
  if (error) {
    // Network error - try fallback
    logError(requestId, error, { provider: provider.name, model });
    
    if (attemptedProviders.length < 3) {
      const fallbackProvider = getNextFallbackProvider(model, registry, modelRouter, attemptedProviders);
      if (fallbackProvider) {
        logProxy(requestId, fallbackProvider.name, model, 'Fallback triggered', {
          originalProvider: provider.name,
          error: error.name
        });
        
        const fallbackUrl = fallbackProvider.baseUrl + fallbackProvider.endpoint;
        const fallbackHeaders = fallbackProvider.buildHeaders();
        const fallbackBody = fallbackProvider.buildBody(body, model);
        
        return tryProviderWithFallback(
          fallbackUrl,
          fallbackHeaders,
          fallbackBody,
          fallbackProvider,
          model,
          registry,
          modelRouter,
          requestId,
          mediaProcessed,
          visionProvider,
          attemptedProviders
        );
      }
    }
    
    return { error: error.name, message: error.message, provider: provider.name, status: 502 };
  }
  
  if (!response.ok) {
    // HTTP error - check if retryable
    const data = await response.json().catch(() => ({}));
    const retryable = isRetryableError(response.status, data);
    
    if (retryable && attemptedProviders.length < 3) {
      const fallbackProvider = getNextFallbackProvider(model, registry, modelRouter, attemptedProviders);
      if (fallbackProvider) {
        logProxy(requestId, fallbackProvider.name, model, 'Fallback triggered', {
          originalProvider: provider.name,
          status: response.status,
          error: retryable
        });
        
        const fallbackUrl = fallbackProvider.baseUrl + fallbackProvider.endpoint;
        const fallbackHeaders = fallbackProvider.buildHeaders();
        const fallbackBody = fallbackProvider.buildBody(body, model);
        
        return tryProviderWithFallback(
          fallbackUrl,
          fallbackHeaders,
          fallbackBody,
          fallbackProvider,
          model,
          registry,
          modelRouter,
          requestId,
          mediaProcessed,
          visionProvider,
          attemptedProviders
        );
      }
    }
    
    return { error: 'Provider error', message: data.error?.message || 'Request failed', provider: provider.name, status: response.status, response };
  }
  
  return { response, provider };
}

/**
 * Get next fallback provider
 */
function getNextFallbackProvider(model, registry, modelRouter, attemptedProviders) {
  const available = registry.getAllProviders();
  const priority = modelRouter.getProviderPriority(model, available);
  
  for (const providerName of priority) {
    if (!attemptedProviders.includes(providerName)) {
      return registry.getProvider(providerName);
    }
  }
  
  return null;
}

/**
 * Forward request to provider
 */
async function forwardRequest(url, headers, body, timeout) {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeout);
  
  try {
    const response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal: controller.signal
    });
    
    clearTimeout(timeoutId);
    return { response, error: null };
  } catch (error) {
    clearTimeout(timeoutId);
    
    if (error.name === 'AbortError') {
      return { response: null, error: { name: 'TIMEOUT', message: 'Request timeout' } };
    }
    
    return { response: null, error: { name: 'NETWORK_ERROR', message: error.message } };
  }
}

/**
 * Check if error is retryable
 */
function isRetryableError(status, data) {
  if (status === 429) return 'RATE_LIMITED';
  if (status >= 500) return 'SERVER_ERROR';
  
  const errorCode = data?.error?.code;
  const errorMessage = data?.error?.message || '';
  
  if (errorCode === '1113' || errorMessage.includes('insufficient balance')) {
    return 'INSUFFICIENT_BALANCE';
  }
  
  return null;
}

/**
 * Send response to client
 */
async function sendResponse(res, response, provider, model, mediaProcessed, visionProvider, requestId) {
  const contentType = response.headers.get('content-type') || '';
  const isSSE = contentType.includes('text/event-stream') || contentType.includes('stream');
  
  if (isSSE) {
    // Stream response
    res.status(response.status);
    for (const [key, value] of response.headers.entries()) {
      if (!['content-encoding', 'content-length', 'transfer-encoding'].includes(key.toLowerCase())) {
        res.setHeader(key, value);
      }
    }
    
    try {
      const reader = response.body.getReader();
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        if (value) res.write(value);
      }
      res.end();
    } catch (streamErr) {
      logError(requestId, streamErr, { provider, model, stage: 'stream' });
      if (!res.headersSent) {
        res.status(502).json({ error: 'Stream failed', requestId });
      }
    }
  } else {
    // Non-streaming response
    const data = await response.json();
    
    logProxy(requestId, provider, model, 'Response received', {
      status: response.status,
      mediaProcessed
    });
    
    res.status(response.status).json({
      ...data,
      _provider: provider,
      _model: model,
      _mediaProcessed: mediaProcessed,
      _visionProvider: visionProvider,
      requestId
    });
  }
}

/**
 * List models
 */
async function handleListModels(req, res) {
  const config = global.config;
  const registry = new ProviderRegistry(config);
  
  const models = [];
  
  for (const provider of registry.providers.values()) {
    const providerModels = provider.config.models || provider.info.defaultModels || [];
    for (const model of providerModels) {
      models.push({
        id: model,
        object: 'model',
        created: Date.now(),
        owned_by: provider.name
      });
    }
  }
  
  res.json({
    object: 'list',
    data: models
  });
}

/**
 * List providers
 */
async function handleListProviders(req, res) {
  const config = global.config;

  const providers = {};

  for (const [name, providerConfig] of Object.entries(config.providers || {})) {
    const info = getProviderInfo(name) || {};
    providers[name] = {
      name,
      displayName: providerConfig.name || info.name || name,
      description: providerConfig.description || info.description,
      type: providerConfig.type || info.type || 'openai',
      capabilities: providerConfig.capabilities || info.capabilities || [],
      models: providerConfig.models || info.defaultModels || [],
      hasApiKey: !!(providerConfig.apiKey && !providerConfig.apiKey.startsWith('YOUR_'))
    };
  }

  res.json(providers);
}

/**
 * Provider health check
 */
async function handleProviderHealth(req, res) {
  const { provider } = req.params;
  const config = global.config;
  const registry = new ProviderRegistry(config);
  
  const prov = registry.getProvider(provider);
  
  if (!prov) {
    return res.status(404).json({ error: 'Provider not found' });
  }
  
  try {
    const testBody = {
      model: prov.config.models?.[0] || prov.info.defaultModels?.[0] || 'test',
      messages: [{ role: 'user', content: 'ping' }],
      max_tokens: 1
    };
    
    const response = await fetch(prov.baseUrl + prov.endpoint, {
      method: 'POST',
      headers: prov.buildHeaders(),
      body: JSON.stringify(testBody)
    });
    
    res.json({
      provider,
      status: response.ok ? 'ok' : 'error',
      statusCode: response.status
    });
  } catch (error) {
    res.json({
      provider,
      status: 'error',
      error: error.message
    });
  }
}

export default router;
