#!/usr/bin/env node

import { bold, cyan, dim, green, red, yellow, magenta } from './setup/colors.js';
import { runWizard, runValidate, runAdvanced } from './setup/index.js';
import { exportConfig, importConfig, writeConfigAtomic } from './setup/configWriter.js';

const args = process.argv.slice(2);
const command = args[0];

const HELP_TEXT = `
${bold(magenta('yi-api-proxy Setup CLI'))}

${bold('Usage:')}
  npm run setup              Start interactive setup wizard
  npm run test:providers     Test provider API connections
  node setup.js --validate   Test providers (non-interactive)
  node setup.js --advanced   Edit config as raw JSON
  node setup.js --export     Export current config to stdout
  node setup.js --import <f> Import and validate config file
  node setup.js --help       Show this help

${bold('Options:')}
  --validate   Test all provider API keys
  --advanced   Edit raw JSON config
  --export     Print current config to stdout
  --import <p> Import config from file path
  --help       Show this help message

${bold('Examples:')}
  npm run setup              # Interactive setup wizard
  npm run test:providers     # Test all providers
  node setup.js --export > backup.json   # Backup config
  node setup.js --import backup.json     # Restore from backup
`;

async function main() {
  switch (command) {
    case '--help':
    case '-h':
    case 'help':
      console.log(HELP_TEXT);
      break;

    case '--validate':
    case 'validate':
      await runValidate();
      break;

    case '--advanced':
    case 'advanced':
      await runAdvanced();
      break;

    case '--export':
      try {
        exportConfig(args[1]);
      } catch (err) {
        console.log(`${red('✗')} ${err.message}`);
        process.exit(1);
      }
      break;

    case '--import': {
      const importPath = args[1];
      if (!importPath) {
        console.log(`${red('✗')} Missing import path. Usage: node setup.js --import <file>`);
        process.exit(1);
      }
      try {
        const config = importConfig(importPath);
        console.log(`${green('✓')} Config imported and validated`);
        const confirm = await askConfirm('Save to config.json?', true);
        if (confirm) {
          writeConfigAtomic(config);
          console.log(`${green('✓')} Saved to config.json`);
        }
      } catch (err) {
        console.log(`${red('✗')} ${err.message}`);
        process.exit(1);
      }
      break;
    }

    case '--wizard':
    case 'wizard':
    case undefined:
      await runWizard();
      break;

    default:
      if (command?.startsWith('--')) {
        console.log(`${red('✗')} Unknown option: ${command}`);
        console.log(`Run ${bold('node setup.js --help')} for usage`);
        process.exit(1);
      } else {
        console.log(`${yellow('⚠')} Use ${bold('npm run setup')} for interactive wizard`);
        console.log(`Run ${bold('node setup.js --help')} for all options`);
      }
  }
}

function askConfirm(question, defaultValue = true) {
  return new Promise((resolve) => {
    const readline = require('readline').createInterface({
      input: process.stdin,
      output: process.stdout
    });
    const yes = defaultValue ? 'Y' : 'y';
    const no = defaultValue ? 'n' : 'N';
    readline.question(`${cyan('?')} ${question} ${dim(`[${yes}/${no}]`)}: `, (answer) => {
      readline.close();
      const a = answer.trim().toLowerCase();
      if (!a) return resolve(defaultValue);
      if (a === 'yes' || a === 'y') return resolve(true);
      if (a === 'no' || a === 'n') return resolve(false);
      resolve(defaultValue);
    });
  });
}

main().catch((err) => {
  console.error(`${red('✗')} Unexpected error: ${err.message}`);
  process.exit(1);
});