const isTTY = process.stdout.isTTY;
const FORCE_COLOR = process.env.FORCE_COLOR === '1';

export function colors(enabled = isTTY || FORCE_COLOR) {
  const none = (x) => x;
  if (!enabled) return { bold: none, red: none, green: none, yellow: none, cyan: none, dim: none, magenta: none };

  const codes = {
    bold: '\x1b[1m',
    red: '\x1b[31m',
    green: '\x1b[32m',
    yellow: '\x1b[33m',
    cyan: '\x1b[36m',
    dim: '\x1b[2m',
    magenta: '\x1b[35m',
    reset: '\x1b[0m'
  };

  const c = (code) => (x) => `${code}${x}${codes.reset}`;
  return {
    bold: c(codes.bold),
    red: c(codes.red),
    green: c(codes.green),
    yellow: c(codes.yellow),
    cyan: c(codes.cyan),
    dim: c(codes.dim),
    magenta: c(codes.magenta)
  };
}

export const clr = colors();
export const bold = clr.bold;
export const red = clr.red;
export const green = clr.green;
export const yellow = clr.yellow;
export const cyan = clr.cyan;
export const dim = clr.dim;
export const magenta = clr.magenta;

export function spinner(msg) {
  const frames = ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'];
  let i = 0;
  const interval = setInterval(() => {
    process.stdout.write(`\r${frames[i++ % frames.length]} ${msg}`);
  }, 80);
  return () => {
    clearInterval(interval);
    process.stdout.write('\r' + ' '.repeat(40) + '\r');
  };
}

export function progress(current, total, label = '') {
  const bar = '█'.repeat(current) + '░'.repeat(total - current);
  process.stdout.write(`\r${label} [${bar}] ${current}/${total}`);
  if (current === total) process.stdout.write('\n');
}

export const eraseLine = '\x1b[2K\r';
export const moveUp = (n) => `\x1b[${n}A`;
export const clearScreen = '\x1b[2J';