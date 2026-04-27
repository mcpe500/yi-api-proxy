/**
 * Proxy Routes - Routes requests to GLM, Z.ai, MiniMax providers
 * 
 * Supports:
 * - GLM (Z.ai Coding Plan) with vision (GLM-5.1)
 * - MiniMax M2.7 (NO vision - images auto-converted via GLM)
 * - Z.ai direct API with Claude models
 * - Automatic fallback to backup provider on retryable errors
 */

import express from 'express';
import { rateLimit } from '../middleware/rateLimit.js';
import { validateApiKey } from '../middleware/auth.js';
import { hasImages, processImages } from '../middleware/mediaHandler.js';
import { log, logProxy, logError } from '../lib/logger.js';

const router = express.Router();

// Retryable error types that trigger fallback
const RETRYABLE_ERRORS = [
  'RATE_LIMITED',
  'SERVER_ERROR', 
  'TIMEOUT',
  'INSUFFICIENT_BALANCE',
  'NETWORK_ERROR'
];

/**
 * POST /v1/chat/completions
 * Universal proxy endpoint
 * 
 * Headers:
 *   X-API-Key: Your API key (required)
 *   X-Provider: glm|zai|minimax (optional, auto-detect from model)
 *   X-Model: Override model (optional)
 * 
 * Body: OpenAI-compatible chat completion format
 */
router.post('/chat/completions', validateApiKey, rateLimit, handleChatCompletions);

router.post('/messages', validateApiKey, rateLimit, handleChatCompletions);

/**
 * GET /v1/models
 * List available models for all or specific provider
 */
router.get('/models', validateApiKey, handleListModels);

/**
 * GET /v1/providers
 * List all providers and their capabilities
 */
router.get('/providers', (req, res) => {
  const config = global.config;
  const providers = Object.entries(config.providers).map(([key, cfg]) => ({
    name: key,
    displayName: cfg.name,
    description: cfg.description,
    models: cfg.models || [],
    visionModels: cfg.visionModels || [],
    hasVision: (cfg.visionModels?.length || 0) > 0,
    capabilities: cfg.capabilities || []
  }));
  res.json({ providers });
});

/**
 * GET /v1/health/:provider
 * Health check for specific provider
 */
router.get('/health/:provider', async (req, res) => {
  const { provider } = req.params;
  const config = global.config;
  const providerConfig = config.providers[provider];
  
  if (!providerConfig) {
    return res.status(404).json({ error: 'Provider not found' });
  }
  
  try {
    const testBody = {
      model: providerConfig.models?.[0] || 'test',
      messages: [{ role: 'user', content: 'ping' }],
      max_tokens: 1
    };
    
    const response = await fetch(providerConfig.baseUrl + providerConfig.endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${providerConfig.apiKey}`
      },
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
});

/**
 * Determine if an error is retryable (should trigger fallback)
 */
function isRetryableError(status, data) {
  // Rate limited
  if (status === 429) return 'RATE_LIMITED';
  
  // Server errors
  if (status >= 500) return 'SERVER_ERROR';
  
  // Check error codes in response body
  const errorCode = data?.error?.code || data?.errorCode;
  const errorMessage = data?.error?.message || data?.errorMessage || '';
  
  // GLM insufficient balance (error 1113)
  if (errorCode === '1113' || errorCode === 1113) return 'INSUFFICIENT_BALANCE';
  if (errorMessage.includes('余额不足') || errorMessage.includes('insufficient balance')) {
    return 'INSUFFICIENT_BALANCE';
  }
  
  // Quota exceeded
  if (errorCode === 'quota_exceeded' || errorMessage.includes('quota')) return 'RATE_LIMITED';
  
  return null;
}

/**
 * Build request body for a provider
 */
function buildRequestBody(processedBody, targetProvider, providerConfig, finalModel) {
  const body = {
    model: finalModel,
    messages: processedBody.messages || processedBody.contents || []
  };

  // Handle Anthropic content format (has content array directly)
  if (processedBody.content && !processedBody.messages) {
    body.messages = processedBody.content;
  }

  // Normalize message content for OpenAI-compatible providers (GLM, Z.ai)
  if (targetProvider !== 'minimax' && body.messages) {
    body.messages = body.messages.map(msg => {
      if (!msg.content) return msg;
      if (typeof msg.content === 'string') return msg;

      if (Array.isArray(msg.content)) {
        const parts = [];
        for (const block of msg.content) {
          if (block.type === 'text' && block.text) {
            parts.push(block.text);
          } else if (block.type === 'tool_result' && block.content) {
            if (typeof block.content === 'string') parts.push(block.content);
            else if (Array.isArray(block.content)) {
              for (const sub of block.content) {
                if (sub.type === 'text' && sub.text) parts.push(sub.text);
              }
            }
          } else if (block.type === 'tool_use' && block.name) {
            parts.push(`[Tool: ${block.name}(${typeof block.input === 'object' ? JSON.stringify(block.input) : block.input || ''})]`);
          } else if (block.type === 'thinking' && block.thinking) {
            // Skip thinking blocks for GLM - not needed
          }
        }
        return { ...msg, content: parts.join('\n') || '' };
      }

      if (typeof msg.content === 'object') {
        return { ...msg, content: JSON.stringify(msg.content) };
      }
      return msg;
    });
  }

  // Handle system field based on provider compatibility
  if (providerConfig.anthropicCompatible) {
    // Anthropic-compatible endpoints (GLM, MiniMax): keep system as top-level field
    if (processedBody.system) {
      if (Array.isArray(processedBody.system)) {
        body.system = processedBody.system
          .filter(c => c.type === 'text')
          .map(c => c.text)
          .join('\n');
      } else {
        body.system = processedBody.system;
      }
    }
    // Extract any system messages from messages array and merge into top-level
    const systemMsgs = body.messages?.filter(m => m.role === 'system') || [];
    if (systemMsgs.length > 0) {
      const extracted = systemMsgs.map(m =>
        typeof m.content === 'string' ? m.content : JSON.stringify(m.content)
      ).join('\n');
      body.system = body.system ? body.system + '\n' + extracted : extracted;
      body.messages = body.messages.filter(m => m.role !== 'system');
    }
  } else {
    // OpenAI-compatible endpoints (Z.ai Direct): convert system to message
    if (processedBody.system) {
      let sysContent = '';
      if (typeof processedBody.system === 'string') {
        sysContent = processedBody.system;
      } else if (Array.isArray(processedBody.system)) {
        sysContent = processedBody.system
          .filter(c => c.type === 'text')
          .map(c => c.text)
          .join('\n');
      }
      if (sysContent) {
        body.messages = [{ role: 'system', content: sysContent }, ...(body.messages || [])];
      }
      delete body.system;
    }
  }

  // Copy other relevant fields
  if (processedBody.temperature) body.temperature = processedBody.temperature;
  if (processedBody.max_tokens) body.max_tokens = processedBody.max_tokens;
  if (processedBody.top_p) body.top_p = processedBody.top_p;
  if (processedBody.stream !== undefined) body.stream = processedBody.stream;
  if (processedBody.stop) body.stop = processedBody.stop;

  return body;
}

/**
 * Forward request to a provider and return response
 */
async function forwardToProvider(targetUrl, headers, body, providerConfig, timeoutMs) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs || 3600000);

  try {
    const response = await fetch(targetUrl, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal: controller.signal
    });

    clearTimeout(timeout);
    return { response, error: null };
  } catch (error) {
    clearTimeout(timeout);
    
    if (error.name === 'AbortError') {
      return { response: null, error: { name: 'TIMEOUT', message: 'Request timeout' } };
    }
    
    if (error.cause?.code === 'ENOTFOUND' || error.cause?.code === 'ECONNREFUSED') {
      return { response: null, error: { name: 'NETWORK_ERROR', message: error.message } };
    }
    
    return { response: null, error: { name: 'NETWORK_ERROR', message: error.message } };
  }
}

/**
 * Send response back to client (handles both streaming and non-streaming)
 */
async function sendResponse(res, response, targetProvider, finalModel, mediaProcessed, visionProvider, requestId, isFallback = false) {
  const contentType = response.headers.get('content-type') || '';
  const isSSE = contentType.includes('text/event-stream') || contentType.includes('stream');

  if (isSSE) {
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
      logError(requestId, streamErr, { provider: targetProvider, model: finalModel, stage: 'stream', fallback: isFallback });
      if (!res.headersSent) {
        res.status(502).json({ error: 'Stream failed', requestId });
      } else {
        res.end();
      }
    }
    return;
  }

  const data = await response.json();

  logProxy(requestId, targetProvider, finalModel, 'Response received', {
    status: response.status,
    mediaProcessed,
    error: data.error,
    errorCode: data.error?.code,
    errorMessage: data.error?.message,
    fallback: isFallback
  });

  res.status(response.status);
  res.json({
    ...data,
    _provider: targetProvider,
    _model: finalModel,
    _mediaProcessed: mediaProcessed,
    _visionProvider: visionProvider,
    _fallback: isFallback,
    requestId
  });
}

async function handleChatCompletions(req, res) {
  const requestId = req.requestId || 'unknown';
  const config = global.config;
  const { provider: requestedProvider, model: requestedModel } = req.headers;
  const requestedModelBody = req.body?.model;
  const targetModel = requestedModel || requestedModelBody;

  logProxy(requestId, requestedProvider || 'auto', targetModel, 'Request received');

  if (!targetModel) {
    logProxy(requestId, requestedProvider || 'auto', '?', 'Missing model');
    return res.status(400).json({ 
      error: 'Missing model',
      message: 'Provide model in body or X-Model header',
      requestId
    });
  }
  
  // Resolve provider and model
  const resolution = resolveProviderAndModel(config, requestedProvider, targetModel);
  if (resolution.error) {
    return res.status(400).json(resolution.error);
  }
  
  let { targetProvider, finalModel } = resolution;
  let providerConfig = config.providers[targetProvider];
  
  // Handle images if present
  let processedBody = req.body;
  let mediaProcessed = false;
  let visionProvider = null;
  
  if (hasImages(req.body)) {
    const hasVision = providerConfig.visionModels?.some(m => finalModel.startsWith(m));
    
    if (!hasVision && config.visionFallback?.enabled) {
      try {
        const { modifiedBody, visionProv } = await processImages(req.body, config, finalModel);
        if (modifiedBody) {
          processedBody = modifiedBody;
          mediaProcessed = true;
          visionProvider = visionProv;
        }
      } catch (error) {
        console.error('Image processing error:', error.message);
      }
    } else if (!hasVision) {
      return res.status(400).json({
        error: 'Provider does not support vision',
        provider: targetProvider,
        model: finalModel,
        hint: 'Use GLM-5.1 for vision support'
      });
    }
  }
  
  // Build headers
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${providerConfig.apiKey}`
  };
  
  if (targetProvider === 'minimax') {
    headers['x-api-key'] = providerConfig.apiKey;
    headers['anthropic-version'] = '2023-06-01';
    delete headers['Authorization'];
  }

  if (providerConfig.extraHeaders) {
    Object.assign(headers, providerConfig.extraHeaders);
  }

  // Build request body
  let body = buildRequestBody(processedBody, targetProvider, providerConfig, finalModel);

  logProxy(requestId, targetProvider, finalModel, 'Body built', {
    msgCount: body.messages?.length,
    msgTypes: body.messages?.slice(0, 5).map(m => ({
      role: m.role,
      contentType: typeof m.content,
      preview: typeof m.content === 'string'
        ? m.content.slice(0, 100)
        : Array.isArray(m.content)
          ? m.content.map(c => c.type).join(',')
          : 'other'
    })),
    stream: body.stream,
    maxTokens: body.max_tokens
  });

  // Try primary provider
  const targetUrl = providerConfig.baseUrl + providerConfig.endpoint;
  logProxy(requestId, targetProvider, finalModel, 'Forwarding request', {
    targetUrl: targetUrl.replace(providerConfig.apiKey, '***'),
    hasImages: hasImages(req.body),
    mediaProcessed
  });

  let { response, error: fetchError } = await forwardToProvider(
    targetUrl, headers, body, providerConfig, providerConfig.timeout
  );

  // Check if we should fallback
  let fallbackTriggered = false;
  let originalError = null;
  
  if (fetchError) {
    // Network/timeout error - check if retryable
    const retryable = fetchError.name; // TIMEOUT or NETWORK_ERROR
    if (RETRYABLE_ERRORS.includes(retryable)) {
      originalError = { type: retryable, message: fetchError.message };
    }
  } else if (!response.ok) {
    // HTTP error - check if retryable
    const data = await response.json().catch(() => ({}));
    const retryable = isRetryableError(response.status, data);
    if (retryable) {
      originalError = { type: retryable, status: response.status, data };
    }
  }

  // Try fallback if primary failed with retryable error
  if (originalError) {
    const fallbackConfig = config.modelFallbacks?.[targetModel];
    
    if (fallbackConfig && fallbackConfig.fallbackProvider && fallbackConfig.fallbackModel) {
      const fallbackProvider = fallbackConfig.fallbackProvider;
      const fallbackModel = fallbackConfig.fallbackModel;
      
      // Prevent infinite loop: don't fallback to same provider
      if (fallbackProvider !== targetProvider) {
        const fallbackProviderConfig = config.providers[fallbackProvider];
        
        if (fallbackProviderConfig) {
          logProxy(requestId, fallbackProvider, fallbackModel, 'Fallback triggered', {
            originalProvider: targetProvider,
            originalModel: finalModel,
            originalError: originalError.type,
            fallback: true
          });

          // Rebuild for fallback provider
          const fallbackHeaders = {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${fallbackProviderConfig.apiKey}`
          };
          
          if (fallbackProvider === 'minimax') {
            fallbackHeaders['x-api-key'] = fallbackProviderConfig.apiKey;
            fallbackHeaders['anthropic-version'] = '2023-06-01';
            delete fallbackHeaders['Authorization'];
          }

          if (fallbackProviderConfig.extraHeaders) {
            Object.assign(fallbackHeaders, fallbackProviderConfig.extraHeaders);
          }

          // Map model for fallback provider
          let fallbackFinalModel = fallbackModel;
          if (fallbackProviderConfig.modelMap?.[fallbackModel]) {
            fallbackFinalModel = fallbackProviderConfig.modelMap[fallbackModel];
          } else if (fallbackProviderConfig.modelMap?.[fallbackModel.toLowerCase()]) {
            fallbackFinalModel = fallbackProviderConfig.modelMap[fallbackModel.toLowerCase()];
          }

          const fallbackBody = buildRequestBody(processedBody, fallbackProvider, fallbackProviderConfig, fallbackFinalModel);
          const fallbackUrl = fallbackProviderConfig.baseUrl + fallbackProviderConfig.endpoint;

          logProxy(requestId, fallbackProvider, fallbackFinalModel, 'Forwarding fallback request', {
            targetUrl: fallbackUrl.replace(fallbackProviderConfig.apiKey, '***'),
            fallback: true
          });

          const fallbackResult = await forwardToProvider(
            fallbackUrl, fallbackHeaders, fallbackBody, fallbackProviderConfig, fallbackProviderConfig.timeout
          );

          if (fallbackResult.response && fallbackResult.response.ok) {
            fallbackTriggered = true;
            response = fallbackResult.response;
            targetProvider = fallbackProvider;
            finalModel = fallbackFinalModel;
            providerConfig = fallbackProviderConfig;
            
            // For fallback, we need to handle vision differently
            // If original had images and fallback doesn't support vision, images were already processed
            // by visionFallback in the primary attempt, so mediaProcessed stays true
          } else {
            // Fallback also failed - log and return original error
            logProxy(requestId, fallbackProvider, fallbackFinalModel, 'Fallback failed', {
              fallback: true,
              error: fallbackResult.error?.message || fallbackResult.response?.status
            });
          }
        }
      }
    }
  }

  // Handle response
  if (response && response.ok) {
    return sendResponse(res, response, targetProvider, finalModel, mediaProcessed, visionProvider, requestId, fallbackTriggered);
  }

  // All attempts failed - return error
  if (res.headersSent) {
    res.end();
    return;
  }

  if (fetchError?.name === 'AbortError' || fetchError?.name === 'TIMEOUT') {
    return res.status(504).json({ 
      error: 'Request timeout', 
      provider: targetProvider,
      timeout: providerConfig.timeout || 3600000,
      requestId
    });
  }

  if (response) {
    const data = await response.json().catch(() => ({}));
    return res.status(response.status).json({
      ...data,
      _provider: targetProvider,
      _model: finalModel,
      _mediaProcessed: mediaProcessed,
      _visionProvider: visionProvider,
      _fallback: fallbackTriggered,
      requestId
    });
  }

  res.status(502).json({ 
    error: 'Proxy request failed',
    message: fetchError?.message || 'Unknown error',
    provider: targetProvider,
    _fallback: fallbackTriggered,
    requestId
  });
}

/**
 * Resolve provider and model from request
 */
function resolveProviderAndModel(config, requestedProvider, targetModel) {
  let targetProvider = requestedProvider?.toLowerCase();
  
  if (!targetProvider) {
    const modelLower = targetModel.toLowerCase();
    
    // Check model aliases
    for (const [alias, provider] of Object.entries(config.aliases || {})) {
      if (modelLower.includes(alias)) {
        targetProvider = provider;
        break;
      }
    }
    
    // Check model prefixes against providers
    if (!targetProvider) {
      for (const [key, cfg] of Object.entries(config.providers)) {
        if (cfg.models?.some(m => modelLower.startsWith(m.toLowerCase())) ||
            cfg.visionModels?.some(m => modelLower.startsWith(m.toLowerCase()))) {
          targetProvider = key;
          break;
        }
      }
    }
    
    // Default to first provider if not detected
    targetProvider = targetProvider || Object.keys(config.providers)[0];
  }
  
  const providerConfig = config.providers[targetProvider];
  
  if (!providerConfig) {
    return {
      error: {
        error: 'Invalid provider',
        available: Object.keys(config.providers),
        hint: 'Try: glm, minimax, or zai'
      }
    };
  }
  
  let finalModel = targetModel;

  if (providerConfig.modelMap?.[targetModel]) {
    finalModel = providerConfig.modelMap[targetModel];
  } else if (providerConfig.modelMap?.[targetModel.toLowerCase()]) {
    finalModel = providerConfig.modelMap[targetModel.toLowerCase()];
  } else {
    const modelLower = targetModel.toLowerCase();
    const canonicalModel = providerConfig.models?.find(m => m.toLowerCase() === modelLower);
    if (canonicalModel && canonicalModel !== targetModel) {
      finalModel = canonicalModel;
    }
  }

  return { targetProvider, finalModel };
}

async function handleListModels(req, res) {
  const requestedProvider = (req.headers['x-provider'] || '').toLowerCase();
  
  if (requestedProvider && global.config.providers[requestedProvider]) {
    const cfg = global.config.providers[requestedProvider];
    return res.json({
      provider: requestedProvider,
      models: cfg.models || [],
      visionModels: cfg.visionModels || []
    });
  }
  
  // Return all models from all providers
  const allModels = {};
  for (const [name, cfg] of Object.entries(global.config.providers)) {
    allModels[name] = {
      models: cfg.models || [],
      visionModels: cfg.visionModels || []
    };
  }
  
  res.json({ providers: allModels });
}

export default router;