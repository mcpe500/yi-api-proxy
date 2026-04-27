/**
 * Security middleware - API key validation
 */

import { log } from '../lib/logger.js';

export function validateApiKey(req, res, next) {
  const config = global.config;

  if (!config.security?.apiKeys || config.security.apiKeys.length === 0) {
    return next();
  }

  const xApiKey = req.headers['x-api-key'];
  const bearer = req.headers['authorization']?.replace('Bearer ', '')?.trim();
  const apiKey = xApiKey || bearer;

  if (apiKey && config.security.apiKeys.includes(apiKey)) {
    return next();
  }

  log('warn', 'Auth failed', {
    path: req.path,
    xApiKey: xApiKey ? `${xApiKey.slice(0, 8)}...` : null,
    bearer: bearer ? `${bearer.slice(0, 8)}...` : null,
    validKeys: config.security.apiKeys.map(k => `${k.slice(0, 8)}...`)
  });

  res.status(401).json({
    error: 'Unauthorized',
    message: 'Invalid or missing API key',
    hint: 'Send X-API-Key header or Authorization: Bearer <key>'
  });
}

/**
 * Validate request origin (optional)
 */
export function validateOrigin(req, res, next) {
  const config = global.config;
  
  // If no allowed origins configured, skip
  if (!config.security?.allowedOrigins?.length) {
    return next();
  }
  
  const origin = req.headers['origin'] || req.headers['referer'];
  
  if (origin && config.security.allowedOrigins.some(o => origin.includes(o))) {
    return next();
  }
  
  // For API requests, allow if no origin header (like curl)
  if (!origin) {
    return next();
  }
  
  res.status(403).json({
    error: 'Forbidden',
    message: 'Origin not allowed'
  });
}