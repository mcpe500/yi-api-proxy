import crypto from 'crypto';
import { dim, green, red, yellow } from './colors.js';

export function validatePort(portStr) {
  const port = parseInt(portStr, 10);
  if (isNaN(port) || port < 1 || port > 65535) {
    return { valid: false, error: `Port must be between 1 and 65535` };
  }
  if (port < 1024) {
    console.log(`  ${yellow('⚠')} Port ${port} < 1024 may require elevated privileges`);
  }
  return { valid: true, value: port };
}

export function validateHost(hostStr) {
  if (!hostStr || hostStr.trim().length === 0) {
    return { valid: false, error: `Host cannot be empty` };
  }
  const trimmed = hostStr.trim();
  const ipPattern = /^(\d{1,3}\.){3}\d{1,3}$/;
  const hostnamePattern = /^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$/;
  if (trimmed === '0.0.0.0' || trimmed === 'localhost' || trimmed === '127.0.0.1') {
    return { valid: true, value: trimmed };
  }
  if (ipPattern.test(trimmed)) {
    const parts = trimmed.split('.').map(Number);
    if (parts.every(p => p >= 0 && p <= 255)) {
      return { valid: true, value: trimmed };
    }
    return { valid: false, error: `Invalid IP address: ${trimmed}` };
  }
  if (hostnamePattern.test(trimmed)) {
    return { valid: true, value: trimmed };
  }
  return { valid: false, error: `Invalid hostname format: ${trimmed}` };
}

export function validateApiKey(apiKey, provider) {
  if (!apiKey || apiKey.trim().length === 0) {
    return { valid: false, error: `API key cannot be empty` };
  }

  const cleaned = apiKey.trim();
  const warnings = [];

  const patterns = {
    glm: /^[a-zA-Z0-9._-]{30,60}$/,
    zai: /^[a-zA-Z0-9._-]{30,60}$/,
    minimax: /^sk-cp-[a-zA-Z0-9_-]{40,}$/
  };

  const pattern = patterns[provider];
  if (pattern && !pattern.test(cleaned)) {
    warnings.push(`${provider.toUpperCase()} API key format may be incorrect`);
  }

  if (cleaned.includes(' ') || cleaned.includes('\n')) {
    return { valid: false, error: `API key contains whitespace or newlines` };
  }

  return {
    valid: true,
    value: cleaned,
    warnings: warnings.length > 0 ? warnings : undefined
  };
}

export function generateApiKey(prefix = 'sk-yi-proxy') {
  const random = crypto.randomBytes(24).toString('hex');
  return `${prefix}-${random}`;
}

const PROVIDER_CONFIGS = {
  glm: {
    name: 'GLM (Z.ai)',
    baseUrl: 'https://api.z.ai/api/anthropic',
    endpoint: '/v1/messages',
    authType: 'bearer'
  },
  minimax: {
    name: 'MiniMax',
    baseUrl: 'https://api.minimax.io',
    endpoint: '/anthropic/v1/messages',
    authType: 'x-api-key'
  },
  zai: {
    name: 'Z.ai Direct',
    baseUrl: 'https://api.z.ai/v1',
    endpoint: '/chat/completions',
    authType: 'bearer'
  }
};

export async function testProvider(provider, apiKey) {
  const config = PROVIDER_CONFIGS[provider];
  if (!config) {
    return { ok: false, error: `Unknown provider: ${provider}`, errorType: 'INVALID_PROVIDER' };
  }

  const url = config.baseUrl + config.endpoint;
  const headers = {
    'Content-Type': 'application/json',
    'anthropic-version': '2023-06-01'
  };

  if (config.authType === 'bearer') {
    headers['Authorization'] = `Bearer ${apiKey}`;
  } else if (config.authType === 'x-api-key') {
    headers['x-api-key'] = apiKey;
  }

  const body = {
    model: provider === 'glm' ? 'glm-4.7' : (provider === 'minimax' ? 'MiniMax-M2.7' : 'glm-4.7'),
    messages: [{ role: 'user', content: 'test' }],
    max_tokens: 1
  };

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 10000);

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal: controller.signal
    });

    clearTimeout(timeout);

    if (response.status === 401 || response.status === 403) {
      const data = await response.json().catch(() => ({}));
      return {
        ok: false,
        error: data?.error?.message || 'Authentication failed',
        errorType: 'AUTH_FAILED',
        status: response.status
      };
    }

    if (response.status === 429) {
      return { ok: false, error: 'Rate limited', errorType: 'RATE_LIMITED', status: 429 };
    }

    if (response.status >= 500) {
      return {
        ok: false,
        error: `Server error: ${response.status}`,
        errorType: 'SERVER_ERROR',
        status: response.status
      };
    }

    if (response.ok) {
      return { ok: true, errorType: 'OK', status: response.status };
    }

    const data = await response.json().catch(() => ({}));
    return {
      ok: false,
      error: data?.error?.message || `Unexpected status: ${response.status}`,
      errorType: 'UNKNOWN',
      status: response.status
    };
  } catch (err) {
    clearTimeout(timeout);

    if (err.name === 'AbortError') {
      return { ok: false, error: 'Connection timed out (10s)', errorType: 'TIMEOUT' };
    }

    if (err.cause?.code === 'ENOTFOUND' || err.cause?.code === 'ECONNREFUSED') {
      return { ok: false, error: 'Network error: could not reach provider', errorType: 'NETWORK_ERROR' };
    }

    return { ok: false, error: err.message, errorType: 'NETWORK_ERROR' };
  }
}

export async function testAllProviders(config) {
  const results = {};
  const providers = Object.keys(config.providers || {});

  for (const name of providers) {
    const prov = config.providers[name];
    if (prov.apiKey) {
      process.stdout.write(`Testing ${name}... `);
      const result = await testProvider(name, prov.apiKey);
      results[name] = result;
      if (result.ok) {
        console.log(`${green('✓')} (${result.status})`);
      } else {
        console.log(`${red('✗')} [${result.errorType}] ${result.error}`);
      }
    } else {
      results[name] = { ok: false, error: 'No API key configured', errorType: 'NOT_CONFIGURED' };
      console.log(`Testing ${name}... ${dim('skipped (no key)')}`);
    }
  }

  return results;
}