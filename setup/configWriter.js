import fs from 'fs';
import path from 'path';
import { green, red, yellow, bold } from './colors.js';

const CONFIG_PATH = path.join(process.cwd(), 'config.json');
const TMP_PATH = path.join(process.cwd(), 'config.json.tmp');
const BACKUP_PATH = path.join(process.cwd(), 'config.json.backup');

export function readConfig() {
  try {
    const content = fs.readFileSync(CONFIG_PATH, 'utf8');
    return JSON.parse(content);
  } catch {
    return null;
  }
}

export function hasConfig() {
  return fs.existsSync(CONFIG_PATH);
}

export function validateSchema(config) {
  const errors = [];

  if (!config.server || typeof config.server !== 'object') {
    errors.push('Missing or invalid server config');
  } else {
    if (!config.server.port || typeof config.server.port !== 'number') {
      errors.push('server.port must be a number');
    }
    if (!config.server.host || typeof config.server.host !== 'string') {
      errors.push('server.host must be a string');
    }
  }

  if (!config.security || typeof config.security !== 'object') {
    errors.push('Missing or invalid security config');
  } else {
    if (!Array.isArray(config.security.apiKeys) || config.security.apiKeys.length === 0) {
      errors.push('security.apiKeys must be a non-empty array');
    }
  }

  if (!config.providers || typeof config.providers !== 'object') {
    errors.push('Missing or invalid providers config');
  } else {
    for (const [name, prov] of Object.entries(config.providers)) {
      if (!prov.baseUrl || typeof prov.baseUrl !== 'string') {
        errors.push(`providers.${name}.baseUrl must be a string`);
      }
      if (!prov.endpoint || typeof prov.endpoint !== 'string') {
        errors.push(`providers.${name}.endpoint must be a string`);
      }
      if (prov.apiKey !== undefined && typeof prov.apiKey !== 'string') {
        errors.push(`providers.${name}.apiKey must be a string or empty`);
      }
    }
  }

  return { valid: errors.length === 0, errors };
}

export function writeConfigAtomic(config) {
  const tmpContent = JSON.stringify(config, null, 2);

  try {
    fs.writeFileSync(TMP_PATH, tmpContent, 'utf8');
  } catch (err) {
    throw new Error(`Failed to write temp file: ${err.message}`);
  }

  let tmpStat;
  try {
    tmpStat = fs.statSync(TMP_PATH);
  } catch {
    throw new Error('Temp file not found after write');
  }

  if (tmpStat.size < tmpContent.length * 0.5) {
    fs.unlinkSync(TMP_PATH);
    throw new Error('Temp file appears truncated');
  }

  try {
    JSON.parse(fs.readFileSync(TMP_PATH, 'utf8'));
  } catch {
    fs.unlinkSync(TMP_PATH);
    throw new Error('Temp file is not valid JSON');
  }

  const targetExists = fs.existsSync(CONFIG_PATH);
  if (targetExists) {
    try {
      fs.copyFileSync(CONFIG_PATH, BACKUP_PATH);
    } catch {
      // backup failed, continue anyway
    }
  }

  try {
    fs.renameSync(TMP_PATH, CONFIG_PATH);
  } catch (err) {
    if (fs.existsSync(TMP_PATH)) {
      try { fs.unlinkSync(TMP_PATH); } catch { /* ignore */ }
    }
    throw new Error(`Failed to rename temp file to config: ${err.message}`);
  }

  return { backupCreated: targetExists };
}

export function rollback() {
  if (fs.existsSync(BACKUP_PATH)) {
    try {
      fs.copyFileSync(BACKUP_PATH, CONFIG_PATH);
      return true;
    } catch {
      return false;
    }
  }
  return false;
}

export function exportConfig(outputPath) {
  const config = readConfig();
  if (!config) {
    throw new Error('No config file found to export');
  }

  const content = JSON.stringify(config, null, 2);
  if (outputPath && outputPath !== '-') {
    fs.writeFileSync(outputPath, content, 'utf8');
    console.log(`${green('✓')} Config exported to ${outputPath}`);
  } else {
    console.log(content);
  }
}

export function importConfig(inputPath) {
  let content;
  try {
    content = fs.readFileSync(inputPath, 'utf8');
  } catch {
    throw new Error(`Could not read file: ${inputPath}`);
  }

  let config;
  try {
    config = JSON.parse(content);
  } catch {
    throw new Error('Invalid JSON in import file');
  }

  const schema = validateSchema(config);
  if (!schema.valid) {
    throw new Error(`Schema validation failed:\n  - ${schema.errors.join('\n  - ')}`);
  }

  return config;
}

export function printConfig(config) {
  console.log('\n' + bold('Generated Configuration:'));
  console.log('─'.repeat(50));
  console.log(`Server: ${config.server.host}:${config.server.port} (${config.server.env || 'development'})`);
  console.log(`Security: ${config.security.apiKeys.length} API key(s), rate limit ${config.security.rateLimit?.maxRequests || '?'}/window`);
  console.log('Providers:');
  for (const [name, prov] of Object.entries(config.providers)) {
    const hasKey = prov.apiKey ? green('✓') : red('✗');
    console.log(`  ${hasKey} ${name}: ${prov.baseUrl}${prov.endpoint}`);
  }
  if (config.visionFallback?.enabled) {
    console.log(`Vision Fallback: ${config.visionFallback.provider}/${config.visionFallback.model}`);
  }
  console.log('─'.repeat(50));
}