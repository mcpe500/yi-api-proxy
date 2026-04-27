import pino from 'pino';
import { mkdirSync, existsSync } from 'fs';
import { join } from 'path';

const LOG_DIR = join(process.cwd(), 'logs');
const PROVIDER_DIR = join(LOG_DIR, 'providers');

const logLevel = process.env.LOG_LEVEL || 'debug';
const isDev = process.env.NODE_ENV !== 'production';

if (!existsSync(LOG_DIR)) mkdirSync(LOG_DIR, { recursive: true });
if (!existsSync(PROVIDER_DIR)) mkdirSync(PROVIDER_DIR, { recursive: true });

const dateStr = () => new Date().toISOString().slice(0, 10).replace(/-/g, '');

function makeLogger(filename) {
  return pino({
    level: logLevel,
    timestamp: () => `,"timestamp":"${new Date().toISOString()}"`,
    formatters: { level: (label) => ({ level: label }) }
  }, pino.destination(filename));
}

const ds = dateStr();
const combined = makeLogger(join(LOG_DIR, `yi-api.${ds}.log`));
const reqLog = makeLogger(join(LOG_DIR, `requests.${ds}.log`));
const provLogs = {};

function getProvLog(provider) {
  if (!provLogs[provider]) {
    provLogs[provider] = makeLogger(join(PROVIDER_DIR, `${provider}.${ds}.log`));
  }
  return provLogs[provider];
}

export function log(level, message, data = {}) {
  combined[level](message, data);
  if (isDev) {
    const tag = level.toUpperCase();
    const fn = level === 'error' ? console.error : level === 'warn' ? console.warn : console.log;
    fn(`[${tag}] ${message}`, Object.keys(data).length ? data : '');
  }
}

export function logRequest(reqId, req, res, ms, extra = {}) {
  const entry = { requestId: reqId, method: req.method, path: req.path, statusCode: res.statusCode, duration: ms, ...extra };
  reqLog.info(entry);
  combined.info(`${req.method} ${req.path} ${res.statusCode} ${ms}ms`, entry);
  if (isDev) {
    const c = res.statusCode >= 500 ? '\x1b[31m' : res.statusCode >= 400 ? '\x1b[33m' : '\x1b[32m';
    console.log(`  ${c}${res.statusCode}\x1b[0m [${reqId}] ${req.method} ${req.path} ${ms}ms`);
  }
}

export function logProxy(reqId, provider, model, action, extra = {}) {
  const data = { requestId: reqId, provider, model, ...extra };
  combined.info(`[${provider}] ${action}`, data);
  getProvLog(provider).info({ ...data, action });
  if (isDev) console.log(`  [\x1b[36m${provider}\x1b[0m] [${reqId}] ${action} (${model})`);
}

export function logError(reqId, error, context = {}) {
  const data = { requestId: reqId, errorName: error.name, ...context };
  combined.error(error.message, data);
  if (context.provider) getProvLog(context.provider).error({ ...data, action: 'error', stack: error.stack?.split('\n').slice(0, 3).join('\n') });
  if (isDev) console.error(`[ERROR] [${reqId}] ${error.message}`, data);
}

export default { log, logRequest, logProxy, logError };
