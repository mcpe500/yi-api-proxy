#!/usr/bin/env node

import { PROVIDER_CATALOG } from './src/config/providerCatalog.js';
import { ModelRouter } from './src/config/modelRouter.js';
import { ProviderRegistry } from './src/providers/providerRegistry.js';
import { openaiToAnthropic, anthropicToOpenai } from './src/core/anthropicProtocol.js';

console.log('✅ All imports successful');
console.log('Providers:', Object.keys(PROVIDER_CATALOG).length);
console.log('Catalog keys:', Object.keys(PROVIDER_CATALOG));
