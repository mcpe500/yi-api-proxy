/**
 * Simple in-memory rate limiter
 */

const limits = new Map();

// Clean expired entries periodically
setInterval(() => {
  const now = Date.now();
  for (const [key, data] of limits.entries()) {
    if (now > data.resetAt) {
      limits.delete(key);
    }
  }
}, 60000); // Clean every minute

export function rateLimit(req, res, next) {
  const config = global.config;
  const windowMs = config.security?.rateLimit?.windowMs || 60000;
  const maxRequests = config.security?.rateLimit?.maxRequests || 100;
  
  // Use API key or IP as rate limit key
  const key = req.headers['x-api-key'] || req.ip || 'anonymous';
  
  const now = Date.now();
  const windowStart = now - windowMs;
  
  if (!limits.has(key)) {
    limits.set(key, { count: 0, resetAt: now + windowMs });
  }
  
  const limit = limits.get(key);
  
  // Reset if window expired
  if (now > limit.resetAt) {
    limit.count = 0;
    limit.resetAt = now + windowMs;
  }
  
  limit.count++;
  
  // Set rate limit headers
  res.setHeader('X-RateLimit-Limit', maxRequests);
  res.setHeader('X-RateLimit-Remaining', Math.max(0, maxRequests - limit.count));
  res.setHeader('X-RateLimit-Reset', Math.ceil(limit.resetAt / 1000));
  
  if (limit.count > maxRequests) {
    return res.status(429).json({
      error: 'Too many requests',
      retryAfter: Math.ceil((limit.resetAt - now) / 1000)
    });
  }
  
  next();
}