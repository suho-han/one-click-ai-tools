#!/usr/bin/env node
const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

// Path to the actual binary (downloaded/placed during install)
const binName = os.platform() === 'win32' ? 'oct.exe' : 'oct';
const binPath = path.join(__dirname, '..', 'bin', binName);

// Fallback to local build for testing if bin doesn't exist
const localBin = path.join(__dirname, '..', 'oct');

let actualPath = null;
if (fs.existsSync(binPath)) {
    actualPath = binPath;
} else if (fs.existsSync(localBin)) {
    actualPath = localBin;
}

if (!actualPath) {
    console.error(`Error: one-click-tools binary not found.`);
    console.error(`Expected location: ${binPath}`);
    console.error(`Please try re-installing: npm install -g one-click-tools`);
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
    console.error(err.message);
    process.exit(1);
});
