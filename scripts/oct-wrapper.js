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
const actualPath = fs.existsSync(binPath) ? binPath : localBin;

const child = spawn(actualPath, process.argv.slice(2), {
    stdio: 'inherit'
});

child.on('exit', (code) => {
    process.exit(code);
});
