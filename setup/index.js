import { bold, cyan, dim, green, red, yellow, magenta } from './colors.js';
import { ask, askConfirm, askSelect, askMultiSelect, askUntilValid } from './questions.js';
import { validatePort, validateHost, validateApiKey, generateApiKey, testProvider, testAllProviders } from './validators.js';
import { hasConfig, readConfig, writeConfigAtomic, rollback, validateSchema, printConfig } from './configWriter.js';

const PROVIDER_INFO = {
  glm: {
    name: 'GLM (Z.ai Coding Plan)',
    description: 'GLM models via Z.ai platform. Supports vision with GLM-5.1.',
    baseUrl: 'https://api.z.ai/api/coding/paas/v4',
    endpoint: '/chat/completions',
    type: 'openai',
    models: ['glm-5.1', 'glm-5-turbo', 'glm-4.7', 'glm-4-flash', 'glm-4-air'],
    visionModels: ['glm-5.1'],
    capabilities: ['vision', 'streaming', 'tools'],
    modelMap: {
      'claude-3-5-sonnet-20240620': 'glm-5-turbo',
      'claude-3-opus-20240229': 'glm-5.1',
      'claude-3-sonnet-20240229': 'glm-5-turbo',
      'gpt-4o': 'glm-5-turbo',
      'gpt-4o-mini': 'glm-4-flash'
    }
  },
  minimax: {
    name: 'MiniMax (Coding Plan)',
    description: 'MiniMax M2.7 via Anthropic-compatible endpoint. NO vision support.',
    baseUrl: 'https://api.minimax.io',
    endpoint: '/anthropic/v1/messages',
    type: 'anthropic',
    models: ['MiniMax-M2.7'],
    visionModels: [],
    capabilities: ['text-only', 'streaming', 'tools'],
    modelMap: {
      'minimax-m2.7': 'MiniMax-M2.7',
      'MiniMax-M2.7-highspeed': 'MiniMax-M2.7'
    }
  },
  zai: {
    name: 'Z.ai (Direct API)',
    description: 'Direct Z.ai API access for Claude models.',
    baseUrl: 'https://api.z.ai/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    models: ['glm-5.1', 'glm-5-turbo'],
    visionModels: ['glm-5.1', 'glm-5v-turbo'],
    capabilities: ['vision', 'streaming', 'tools']
  },
  nvidia_nim: {
    name: 'NVIDIA NIM',
    description: 'NVIDIA NIM inference platform. Supports various models including GLM.',
    baseUrl: 'https://integrate.api.nvidia.com/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    models: ['nvidia_nim/z-ai/glm4.7', 'nvidia_nim/meta/llama-3.1-405b-instruct'],
    visionModels: ['nvidia_nim/z-ai/glm4.7'],
    capabilities: ['vision', 'streaming', 'tools'],
    modelMap: {
      'claude-3-5-sonnet': 'nvidia_nim/z-ai/glm4.7',
      'claude-3-opus': 'nvidia_nim/z-ai/glm4.7',
      'claude-3-sonnet': 'nvidia_nim/z-ai/glm4.7'
    }
  },
  openrouter: {
    name: 'OpenRouter',
    description: 'OpenRouter aggregates many AI models. Supports Claude, GPT, and more.',
    baseUrl: 'https://openrouter.ai/api/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    models: ['anthropic/claude-3.5-sonnet', 'anthropic/claude-3-opus', 'openai/gpt-4o'],
    visionModels: ['anthropic/claude-3.5-sonnet', 'anthropic/claude-3-opus', 'openai/gpt-4o'],
    capabilities: ['vision', 'streaming', 'tools'],
    modelMap: {
      'claude-3-5-sonnet': 'anthropic/claude-3.5-sonnet',
      'claude-3-opus': 'anthropic/claude-3-opus',
      'claude-3-sonnet': 'anthropic/claude-3-sonnet',
      'gpt-4o': 'openai/gpt-4o',
      'gpt-4o-mini': 'openai/gpt-4o-mini'
    }
  },
  deepseek: {
    name: 'DeepSeek',
    description: 'DeepSeek AI models. Cost-effective alternative.',
    baseUrl: 'https://api.deepseek.com/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    models: ['deepseek-chat', 'deepseek-coder'],
    visionModels: [],
    capabilities: ['text-only', 'streaming', 'tools'],
    modelMap: {
      'claude-3-sonnet': 'deepseek-chat',
      'gpt-4': 'deepseek-chat',
      'gpt-4-turbo': 'deepseek-chat'
    }
  },
  lm_studio: {
    name: 'LM Studio (Local)',
    description: 'Local LM Studio server. Run models locally.',
    baseUrl: 'http://localhost:1234/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    models: ['local-model'],
    visionModels: ['local-model'],
    capabilities: ['vision', 'streaming', 'tools'],
    modelMap: {}
  },
  llama_cpp: {
    name: 'llama.cpp (Local)',
    description: 'Local llama.cpp server. Run quantized models locally.',
    baseUrl: 'http://localhost:8080/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    models: ['local-model'],
    visionModels: [],
    capabilities: ['text-only', 'streaming'],
    modelMap: {}
  },
  ollama: {
    name: 'Ollama (Local)',
    description: 'Local Ollama server. Run open-source models locally.',
    baseUrl: 'http://localhost:11434/v1',
    endpoint: '/chat/completions',
    type: 'openai',
    models: ['llama3', 'llama3:70b', 'mistral', 'codellama'],
    visionModels: ['llava', 'llava:13b'],
    capabilities: ['vision', 'streaming', 'tools'],
    modelMap: {
      'claude-3-sonnet': 'llama3:70b',
      'gpt-4': 'llama3:70b',
      'gpt-4-turbo': 'llama3:70b'
    }
  }
};

const VISION_FALLBACK_MODEL = 'glm-5.1';

function stepHeader(text) {
  console.log('\n' + bold(cyan(text)));
}

function sep(text = '') {
  const line = dim('─'.repeat(40));
  console.log(magenta('─') + ' ' + text + ' ' + line);
}

export async function runWizard() {
  if (!process.stdin.isTTY) {
    console.error(red('X') + ' Interactive mode requires a terminal. Use --validate or --advanced instead.');
    process.exit(1);
  }

  const border = magenta('*').repeat(50);
  console.log('\n' + bold(magenta('========================================')));
  console.log(bold(magenta('     yi-api-proxy Setup Wizard')));
  console.log(bold(magenta('========================================')));

  const existingConfig = hasConfig() ? readConfig() : null;
  if (existingConfig) {
    console.log('\n' + yellow('!') + ' Existing config.json detected');
    const overwrite = await askConfirm('Overwrite existing config?', false);
    if (!overwrite) {
      console.log('Setup cancelled.');
      return;
    }
  }

  stepHeader('Step 1: Server Configuration');
  const server = {};
  const portResult = await askUntilValid('Port', validatePort, '3000');
  server.port = portResult || 3000;
  const hostResult = await askUntilValid('Host', validateHost, '0.0.0.0');
  server.host = hostResult || '0.0.0.0';
  const envIdx = await askSelect('Environment', ['development', 'production'], 0);
  server.env = envIdx === 0 ? 'development' : 'production';

  stepHeader('Step 2: Security');
  const generateKey = await askConfirm('Generate a new proxy API key?', true);
  const security = { apiKeys: [], rateLimit: { windowMs: 60000, maxRequests: 200 } };

  if (generateKey) {
    const newKey = generateApiKey();
    security.apiKeys = [newKey];
    console.log('\n' + green('V') + ' Generated API key: ' + bold(newKey));
    const save = await askConfirm('Save this key - you will need it to access the proxy', true);
    if (!save) {
      security.apiKeys = [];
    }
  } else {
    const keyPrompt = await ask('Enter proxy API key(s), comma-separated');
    if (keyPrompt.trim()) {
      security.apiKeys = keyPrompt.split(',').map(k => k.trim()).filter(Boolean);
    }
  }

  stepHeader('Step 3: Providers');
  const providerOptions = Object.entries(PROVIDER_INFO).map(([key, info]) => info.name);
  const selectedIndices = await askMultiSelect('Select providers to configure:', providerOptions);

  if (selectedIndices.length === 0) {
    console.log(red('X') + ' At least one provider must be selected.');
    return;
  }

  const selectedProviders = selectedIndices.map(i => Object.keys(PROVIDER_INFO)[i]);
  const providers = {};

  const timeoutAnswer = await ask('Request timeout (ms, default 3600000 = 1 hour)', '3600000');
  const configuredTimeout = parseInt(timeoutAnswer, 10) || 3600000;

  for (const key of selectedProviders) {
    const info = PROVIDER_INFO[key];
    const sepLine = dim('─'.repeat(40));
    console.log('\n' + magenta('-') + ' ' + info.name + ' ' + sepLine);
    console.log(dim('URL: ' + info.baseUrl + info.endpoint));
    console.log(dim('Models: ' + info.models.join(', ')));

    const apiKeyResult = await askUntilValid(
      'API key for ' + info.name,
      (input) => validateApiKey(input, key),
      ''
    );

    if (!apiKeyResult) {
      console.log(yellow('!') + ' Skipping ' + info.name + ' - no API key');
      continue;
    }

    const validated = await validateApiKey(apiKeyResult, key);
    if (validated.warnings) {
      for (const w of validated.warnings) {
        console.log('  ' + yellow('!') + ' ' + w);
      }
    }

    providers[key] = {
      name: info.name,
      description: info.description,
      baseUrl: info.baseUrl,
      endpoint: info.endpoint,
      type: info.type,
      apiKey: validated.value,
      timeout: configuredTimeout,
      models: info.models,
      visionModels: info.visionModels,
      capabilities: info.capabilities
    };

    if (info.modelMap && Object.keys(info.modelMap).length > 0) {
      providers[key].modelMap = info.modelMap;
    }
  }

  if (Object.keys(providers).length === 0) {
    console.log(red('X') + ' No providers configured. Setup cancelled.');
    return;
  }

  stepHeader('Step 4: Connectivity Test');
  console.log(dim('Testing API keys (this may take a few seconds)...\n'));

  for (const [key, prov] of Object.entries(providers)) {
    if (!prov.apiKey) continue;
    process.stdout.write('  ' + prov.name + ': ');
    const result = await testProvider(key, prov.apiKey);
    if (result.ok) {
      console.log(green('V') + ' Connected (' + result.status + ')');
    } else {
      console.log(red('X') + ' [' + result.errorType + '] ' + result.error);
      const continueAnyway = await askConfirm('Continue with ' + prov.name + ' despite error?', false);
      if (!continueAnyway) {
        delete providers[key];
      }
    }
  }

  stepHeader('Step 5: Vision Fallback');
  const enableFallback = await askConfirm('Enable vision fallback for non-vision providers?', true);
  const visionFallback = { enabled: false };

  if (enableFallback) {
    visionFallback.enabled = true;
    visionFallback.provider = 'glm';
    visionFallback.model = VISION_FALLBACK_MODEL;
    visionFallback.description = 'When non-vision provider receives images, convert using GLM-5.1 vision';
    console.log(green('V') + ' Vision fallback enabled using GLM-5.1');
  }

  stepHeader('Step 6: Model Routing');
  const enableModelRouting = await askConfirm('Enable per-model routing (Opus/Sonnet/Haiku tiers)?', true);
  const modelRouting = { enabled: false };

  if (enableModelRouting) {
    modelRouting.enabled = true;
    console.log(dim('Model routing will automatically route:'));
    console.log(dim('  Opus tier (claude-3-opus, gpt-4) → GLM → NVIDIA NIM → OpenRouter'));
    console.log(dim('  Sonnet tier (claude-3-sonnet, gpt-4o) → GLM → NVIDIA NIM → MiniMax'));
    console.log(dim('  Haiku tier (claude-3-haiku, gpt-4o-mini) → MiniMax → GLM → DeepSeek'));
  }

  stepHeader('Step 7: Model Aliases');
  const aliases = {
    'claude-opus': 'glm',
    'claude-sonnet': 'glm',
    'claude-haiku': 'glm',
    'gpt-4': 'glm',
    'minimax': 'minimax'
  };

  const addAliases = await askConfirm('Add default model aliases?', true);
  if (!addAliases) {
    Object.keys(aliases).forEach(k => delete aliases[k]);
  }

  const config = {
    server,
    security,
    enableModelRouting: enableModelRouting,
    modelRouting: enableModelRouting ? {
      opus: {
        models: ['claude-3-opus', 'claude-3.5-opus', 'gpt-4', 'gpt-4-turbo'],
        primaryProvider: 'glm',
        fallbackProviders: ['nvidia_nim', 'openrouter', 'zai']
      },
      sonnet: {
        models: ['claude-3-sonnet', 'claude-3.5-sonnet', 'gpt-4o', 'glm-5-turbo'],
        primaryProvider: 'glm',
        fallbackProviders: ['nvidia_nim', 'openrouter', 'minimax']
      },
      haiku: {
        models: ['claude-3-haiku', 'gpt-4o-mini', 'glm-4-flash', 'glm-4-air'],
        primaryProvider: 'minimax',
        fallbackProviders: ['glm', 'deepseek', 'ollama']
      }
    } : undefined,
    providers,
    visionFallback,
    aliases
  };

  printConfig(config);

  const confirm = await askConfirm('\nWrite this configuration?', true);
  if (!confirm) {
    console.log('Setup cancelled.');
    return;
  }

  console.log('\nWriting config.json...');
  try {
    const result = writeConfigAtomic(config);
    console.log(green('V') + ' Configuration saved to config.json');
    if (result.backupCreated) {
      console.log(dim('  Backup created: config.json.backup'));
    }
  } catch (err) {
    console.log(red('X') + ' Failed to write config: ' + err.message);
    console.log('Attempting rollback...');
    if (rollback()) {
      console.log(yellow('!') + ' Rollback successful, previous config restored');
    } else {
      console.log(red('X') + ' Rollback failed, previous config may be lost');
    }
    return;
  }

  stepHeader('Step 8: Post-Setup Validation');
  const results = await testAllProviders(config);

  const allOk = Object.values(results).every(r => r.ok);
  if (allOk) {
    console.log('\n' + green('V') + ' All providers connected successfully!');
  } else {
    const failed = Object.entries(results).filter(([, r]) => !r.ok).map(([k]) => k);
    console.log('\n' + yellow('!') + ' Some providers failed: ' + failed.join(', '));
    console.log('Run ' + bold('npm run test:providers') + ' to re-test later');
  }

  console.log('\n' + bold(green('Setup complete!')));
  console.log('Run ' + bold('npm run dev') + ' to start the proxy server');
}

export async function runValidate() {
  if (!hasConfig()) {
    console.log(yellow('!') + ' No config.json found. Run ' + bold('npm run setup') + ' first.');
    return;
  }

  const config = readConfig();
  console.log('\n' + bold(cyan('Testing Provider Connections')));
  console.log(dim('-'.repeat(50)));

  const results = await testAllProviders(config);

  console.log(dim('-'.repeat(50)));
  const allOk = Object.values(results).every(r => r.ok);
  if (allOk) {
    console.log(green('V') + ' All providers connected');
  } else {
    const failed = Object.entries(results).filter(([, r]) => !r.ok);
    console.log(red('X') + ' ' + failed.length + ' provider(s) failed:');
    for (const [name, result] of failed) {
      console.log('  ' + red('X') + ' ' + name + ': [' + result.errorType + '] ' + result.error);
    }
  }
}

export async function runAdvanced() {
  if (!process.stdin.isTTY) {
    console.error(red('X') + ' Advanced mode requires a terminal.');
    process.exit(1);
  }

  const existing = hasConfig() ? readConfig() : null;
  const editor = await ask('Enter full JSON configuration (or press Enter to use existing)');

  let config;
  if (editor.trim()) {
    try {
      config = JSON.parse(editor);
    } catch {
      console.log(red('X') + ' Invalid JSON');
      return;
    }
  } else if (existing) {
    config = existing;
    console.log('Using existing config.');
  } else {
    console.log(yellow('!') + ' No existing config and no new JSON provided.');
    return;
  }

  const schema = validateSchema(config);
  if (!schema.valid) {
    console.log(red('X') + ' Schema validation failed:');
    for (const err of schema.errors) console.log('  - ' + err);
    return;
  }

  const save = await askConfirm('Save this configuration?', true);
  if (!save) {
    console.log('Cancelled.');
    return;
  }

  try {
    writeConfigAtomic(config);
    console.log(green('V') + ' Config saved.');
  } catch (err) {
    console.log(red('X') + ' Write failed: ' + err.message);
  }
}