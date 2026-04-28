/**
 * Yi API Proxy - Modular AI API Proxy
 * Inspired by free-claude-code architecture
 * Supports: GLM, Z.ai, MiniMax, NVIDIA NIM, OpenRouter, DeepSeek, LM Studio, llama.cpp, Ollama
 */

import express from 'express';
import cors from 'cors';
import helmet from 'helmet';
import crypto from 'crypto';
import { readFileSync, existsSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import { log, logRequest, logError } from './lib/logger.js';
import { validateApiKey } from './middleware/auth.js';
import { rateLimit } from './middleware/rateLimit.js';
import chatRoutes from './api/chatRoutes.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Load config.json (project root, not src/)
const configPath = join(__dirname, '..', 'config.json');

if (!existsSync(configPath)) {
  console.error('❌ config.json not found!');
  console.error('   Copy config.example.json to config.json and add your API keys');
  process.exit(1);
}

let config;
try {
  config = JSON.parse(readFileSync(configPath, 'utf-8'));
} catch (e) {
  console.error('❌ Failed to parse config.json:', e.message);
  process.exit(1);
}

// Validate config has required fields
const requiredFields = ['providers'];
for (const field of requiredFields) {
  if (!config[field]) {
    console.error(`❌ Invalid config: missing ${field}`);
    process.exit(1);
  }
}

// Validate each provider has apiKey
for (const [name, provider] of Object.entries(config.providers)) {
  if (!provider.apiKey || provider.apiKey === `YOUR_${name.toUpperCase()}_API_KEY`) {
    console.warn(`⚠️ Provider "${name}" missing or has placeholder apiKey`);
  }
}

if (!config.security?.apiKeys?.length) {
  console.warn('⚠️ Warning: No security.apiKeys configured. Add keys in config.json');
}

// Set global config for middleware access
global.config = config;

const app = express();
const PORT = config.server?.port || 3000;
const HOST = config.server?.host || '0.0.0.0';

// Request ID middleware
app.use((req, res, next) => {
  const requestId = req.headers['x-request-id'] || crypto.randomUUID().slice(0, 8);
  req.requestId = requestId;
  res.setHeader('X-Request-ID', requestId);
  next();
});

// Middleware
app.use(helmet({
  contentSecurityPolicy: false,
  crossOriginEmbedderPolicy: false
}));

app.use(cors({
  origin: config.server?.corsOrigins || '*',
  methods: ['GET', 'POST', 'PUT', 'DELETE', 'OPTIONS'],
  allowedHeaders: ['Content-Type', 'Authorization', 'X-API-Key', 'X-Provider', 'X-Model', 'X-Request-ID']
}));

app.use(express.json({ limit: '50mb' }));

// Request logging — always log to file + console in dev
app.use((req, res, next) => {
  const start = Date.now();
  res.on('finish', () => {
    const ms = Date.now() - start;
    logRequest(req.requestId, req, res, ms);
  });
  next();
});

// Health check
app.get('/health', (req, res) => {
  res.json({ 
    status: 'ok', 
    providers: Object.keys(config.providers),
    timestamp: new Date().toISOString(),
    requestId: req.requestId
  });
});

// Readiness check (lightweight - for k8s/load balancers)
app.get('/ready', (req, res) => {
  res.json({
    ready: true,
    timestamp: new Date().toISOString(),
    requestId: req.requestId
  });
});

// Info endpoint
app.get('/', (req, res) => {
  res.json({
    name: 'Yi API Proxy',
    version: '2.0.0',
    description: 'Modular AI API Proxy with multi-provider support and per-model routing',
    endpoints: {
      chat: 'POST /v1/chat/completions',
      messages: 'POST /v1/messages',
      models: 'GET /v1/models',
      providers: 'GET /v1/providers',
      health: 'GET /health',
      ready: 'GET /ready'
    },
    features: [
      'Multi-provider routing (GLM, Z.ai, MiniMax, NVIDIA NIM, OpenRouter, DeepSeek, LM Studio, llama.cpp, Ollama)',
      'Per-model routing (Opus, Sonnet, Haiku tiers)',
      'Automatic fallback between providers',
      'Vision support with automatic fallback',
      'Streaming support',
      'Tool use support',
      'Thinking/reasoning block handling'
    ],
    providers: Object.entries(config.providers).map(([key, cfg]) => ({
      name: key,
      displayName: cfg.name,
      description: cfg.description,
      models: cfg.models || [],
      visionModels: cfg.visionModels || [],
      hasVision: (cfg.visionModels?.length || 0) > 0,
      capabilities: cfg.capabilities || []
    })),
    requestId: req.requestId
  });
});

// Apply auth and rate limiting to /v1 routes
app.use('/v1', validateApiKey, rateLimit);

// Use chat routes
app.use('/v1', chatRoutes);

// 404 handler
app.use((req, res) => {
  res.status(404).json({ 
    error: 'Not found', 
    path: req.path,
    requestId: req.requestId,
    hint: 'POST /v1/chat/completions | GET /v1/providers | GET /health'
  });
});

// Error handler with request ID
app.use((err, req, res, next) => {
  const requestId = req.requestId || 'unknown';
  logError(requestId, err, { path: req.path });
  
  const status = err.status || 500;
  const message = status === 500 ? 'Internal server error' : err.message;
  
  res.status(status).json({ 
    error: message,
    requestId,
    path: req.path,
    ...(process.env.NODE_ENV !== 'production' && { stack: err.stack?.split('\n').slice(0, 3) })
  });
});

app.listen(PORT, HOST, () => {
  const providerStatus = Object.entries(config.providers).map(([name, cfg]) => {
    const hasApiKey = cfg.apiKey && !cfg.apiKey.startsWith('YOUR_');
    return { name, displayName: cfg.name || name, models: cfg.models || [], hasApiKey, visionModels: cfg.visionModels || [] };
  });

  log('info', 'Server started', { port: PORT, host: HOST, providers: providerStatus.map(p => p.name) });

  for (const [name, cfg] of Object.entries(config.providers)) {
    if (!cfg.apiKey || cfg.apiKey.startsWith('YOUR_')) {
      log('warn', `Provider "${name}" missing or placeholder apiKey`);
    }
  }

  console.log('\n🚀 Yi API Proxy v2.0');
  console.log(`   Server: http://localhost:${PORT}`);
  console.log(`   Health: http://localhost:${PORT}/health`);
  console.log(`   Logs:   logs/yi-api.${new Date().toISOString().slice(0, 10).replace(/-/g, '')}.log`);
  console.log('\n📦 Providers:');
  for (const [name, cfg] of Object.entries(config.providers)) {
    const hasApiKey = cfg.apiKey && !cfg.apiKey.startsWith('YOUR_');
    const statusIcon = hasApiKey ? '✅' : '⚠️ ';
    console.log(`   ${statusIcon} ${name} (${cfg.name || name})`);
    console.log(`        Models: ${(cfg.models || []).slice(0, 3).join(', ')}${(cfg.models?.length || 0) > 3 ? '...' : ''}`);
    console.log(`        Vision: ${cfg.visionModels?.length ? cfg.visionModels.join(', ') : 'No'}`);
  }
  console.log('\n🔑 Security API Keys:', config.security?.apiKeys?.length || 0);
  console.log('📁 Log files: logs/');
  console.log('\n📖 Usage:');
  console.log('   POST /v1/chat/completions');
  console.log('   Header: X-API-Key: your-key');
  console.log('   Header: X-Provider: glm|zai|minimax|nvidia_nim|openrouter|deepseek (auto-detect if omitted)');
  console.log('   Body: OpenAI-compatible format');
  console.log('\n🎯 Model Routing:', config.enableModelRouting !== false ? 'Enabled (Opus/Sonnet/Haiku tiers)' : 'Disabled');
  console.log('   Use X-Provider header to override routing\n');
});

export default app;