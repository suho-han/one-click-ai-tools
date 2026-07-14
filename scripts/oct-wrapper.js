#!/usr/bin/env node
const { spawn, spawnSync } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

// Path to the actual binary (downloaded/placed during install)
const binName = os.platform() === 'win32' ? 'oct.exe' : 'oct';
const binPath = path.join(__dirname, '..', 'bin', binName);
const postinstallScript = path.join(__dirname, 'postinstall.js');

// Optional local fallback for development only.
// In global npm installs, a bundled repo-local ./oct can be wrong-arch and trigger ENOEXEC.
const localBin = path.join(__dirname, '..', 'oct');
const allowLocalFallback = process.env.OCT_WRAPPER_ALLOW_LOCAL_BIN === '1';

function canExecute(p) {
  try {
    fs.accessSync(p, fs.constants.X_OK);
    return true;
  } catch {
    return false;
  }
}

function resolveBinaryPath() {
  if (canExecute(binPath)) return binPath;
  if (allowLocalFallback && canExecute(localBin)) return localBin;
  return null;
}

function ensureBinaryInstalled() {
  if (!fs.existsSync(postinstallScript)) return false;

  const result = spawnSync(process.execPath, [postinstallScript], {
    stdio: 'inherit',
    cwd: os.homedir(),
    env: process.env,
  });

  return result.status === 0 && canExecute(binPath);
}

let actualPath = resolveBinaryPath();

if (!actualPath) {
  console.error(`Warning: one-click-ai-tools binary not found at ${binPath}`);
  console.error('Attempting self-heal by running postinstall...');

  const healed = ensureBinaryInstalled();
  if (healed) {
    actualPath = resolveBinaryPath();
  }
}

if (!actualPath) {
  console.error(`Error: one-click-ai-tools binary not found.`);
  console.error(`Expected location: ${binPath}`);
  console.error(`Current npm prefix: ${process.env.npm_config_prefix || '(unknown)'}`);
  console.error(`Please try re-installing: npm install -g one-click-ai-tools`);
  console.error(`Dev-only local fallback is disabled by default. Set OCT_WRAPPER_ALLOW_LOCAL_BIN=1 to enable it.`);
  process.exit(1);
}

const child = spawn(actualPath, process.argv.slice(2), {
  stdio: 'inherit'
});

child.on('exit', (code) => {
  process.exit(code);
});

child.on('error', (err) => {
  console.error(`Error: Failed to start the one-click-ai-tools binary.`);
  if (err && err.code === 'ENOEXEC') {
    console.error(`Detected executable format mismatch (ENOEXEC).`);
    console.error(`This usually means a wrong-architecture binary was installed for this OS/CPU.`);
    console.error(`Try reinstalling: npm uninstall -g one-click-ai-tools && npm install -g one-click-ai-tools`);
  }
  console.error(err.message);
  process.exit(1);
});
