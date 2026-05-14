#!/usr/bin/env node
const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

// Path to the actual binary (downloaded/placed during install)
const binName = os.platform() === 'win32' ? 'oct.exe' : 'oct';
const binPath = path.join(__dirname, '..', 'bin', binName);

// Optional local fallback for development only.
// In global npm installs, a bundled repo-local ./oct can be wrong-arch and trigger ENOEXEC.
const localBin = path.join(__dirname, '..', 'oct');
const allowLocalFallback = process.env.OCT_WRAPPER_ALLOW_LOCAL_BIN === '1';

let actualPath = null;
if (fs.existsSync(binPath)) {
    actualPath = binPath;
} else if (allowLocalFallback && fs.existsSync(localBin)) {
    actualPath = localBin;
}

if (!actualPath) {
    console.error(`Error: one-click-tools binary not found.`);
    console.error(`Expected location: ${binPath}`);
    console.error(`Please try re-installing: npm install -g one-click-tools`);
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
    console.error(`Error: Failed to start the one-click-tools binary.`);
    if (err && err.code === 'ENOEXEC') {
        console.error(`Detected executable format mismatch (ENOEXEC).`);
        console.error(`This usually means a wrong-architecture binary was installed for this OS/CPU.`);
        console.error(`Try reinstalling: npm uninstall -g one-click-tools && npm install -g one-click-tools`);
    }
    console.error(err.message);
    process.exit(1);
});
