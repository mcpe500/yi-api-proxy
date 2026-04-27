import readline from 'readline';
import { bold, cyan, dim } from './colors.js';

export function createInterface() {
  return readline.createInterface({
    input: process.stdin,
    output: process.stdout
  });
}

export function ask(question, defaultValue = '') {
  return new Promise((resolve) => {
    const rl = createInterface();
    const prompt = defaultValue
      ? `${cyan('?')} ${question} ${dim(`[${defaultValue}]`)}: `
      : `${cyan('?')} ${question}: `;

    rl.question(prompt, (answer) => {
      rl.close();
      resolve(answer.trim() || defaultValue);
    });
  });
}

export function askPassword(question) {
  return new Promise((resolve) => {
    const rl = createInterface({ terminal: true });
    const silent = readline.createInterface({
      input: process.stdin,
      output: process.stdout
    });

    rl.question(`${cyan('?')} ${question} (hidden): `, (answer) => {
      rl.close();
      process.stdout.write('\n');
      resolve(answer);
    });
  });
}

export function askConfirm(question, defaultValue = true) {
  return new Promise((resolve) => {
    const rl = createInterface();
    const yes = defaultValue ? 'Y' : 'y';
    const no = defaultValue ? 'n' : 'N';
    const hint = `[${yes}/${no}]`;

    rl.question(`${cyan('?')} ${question} ${dim(hint)}: `, (answer) => {
      rl.close();
      const a = answer.trim().toLowerCase();
      if (!a) return resolve(defaultValue);
      if (a === 'yes' || a === 'y') return resolve(true);
      if (a === 'no' || a === 'n') return resolve(false);
      resolve(defaultValue);
    });
  });
}

export function askSelect(question, options, defaultIdx = 0) {
  return new Promise((resolve) => {
    const rl = createInterface();
    console.log(`\n${bold(question)}`);
    options.forEach((opt, i) => {
      const marker = i === defaultIdx ? cyan('►') : ' ';
      console.log(`  ${marker} ${i + 1}. ${opt}`);
    });
    console.log();

    const prompt = `Enter number`;
    rl.question(`${cyan('?')} ${prompt} ${dim(`[${defaultIdx + 1}]`)}: `, (answer) => {
      rl.close();
      const idx = parseInt(answer, 10) - 1;
      if (isNaN(idx) || idx < 0 || idx >= options.length) {
        return resolve(defaultIdx);
      }
      resolve(idx);
    });
  });
}

export function askMultiSelect(question, options) {
  return new Promise((resolve) => {
    const rl = createInterface();
    console.log(`\n${bold(question)}`);
    console.log(`${dim('Enter numbers separated by comma/space, or "all"')}\n`);
    options.forEach((opt, i) => {
      console.log(`   ${cyan(String(i + 1).padStart(2, ' '))}. ${opt}`);
    });
    console.log();

    rl.question(`${cyan('?')} Select: `, (answer) => {
      rl.close();
      const trimmed = answer.trim().toLowerCase();

      if (trimmed === 'all') {
        return resolve(options.map((_, i) => i));
      }

      const parts = trimmed.split(/[\s,]+/).filter(Boolean);
      const indices = parts
        .map(p => parseInt(p, 10) - 1)
        .filter(i => !isNaN(i) && i >= 0 && i < options.length);

      if (indices.length === 0) {
        return resolve([]);
      }
      resolve([...new Set(indices)].sort((a, b) => a - b));
    });
  });
}

export async function askRetry(question, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    const answer = await ask(`${question} (attempt ${i + 1}/${maxRetries})`);
    if (answer) return answer;
  }
  return null;
}

export async function askUntilValid(question, validator, defaultValue = '') {
  let attempt = 0;
  while (attempt < 5) {
    const answer = await ask(question, defaultValue);
    const result = await validator(answer);
    if (result.valid) return result.value;
    console.log(`  ${red('✗')} ${result.error}`);
    attempt++;
  }
  return null;
}