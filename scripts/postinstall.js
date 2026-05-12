const os = require('os');
const fs = require('fs');
const path = require('path');
const https = require('https');
const { execSync } = require('child_process');

const packageJson = require(path.join(__dirname, '..', 'package.json'));
const version = `v${packageJson.version}`;
const repo = 'suho-han/one-click-tools';

const platformMap = {
    'darwin': 'darwin',
    'linux': 'linux',
    'win32': 'windows'
};

const archMap = {
    'x64': 'amd64',
    'arm64': 'arm64'
};

const platform = platformMap[os.platform()];
const arch = archMap[os.arch()];

if (!platform || !arch) {
    console.error(`Unsupported platform/architecture: ${os.platform()}/${os.arch()}`);
    process.exit(1);
}

const binName = platform === 'windows' ? 'oct.exe' : 'oct';
const binaryNameInArchive = binName;
const archiveName = `one-click-tools_${platform}_${arch}${platform === 'windows' ? '.zip' : '.tar.gz'}`;
const downloadUrl = `https://github.com/${repo}/releases/download/${version}/${archiveName}`;

const binDir = path.join(__dirname, '..', 'bin');
if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
}

const targetPath = path.join(binDir, binName);
const homeDir = os.homedir();
const octDir = path.join(homeDir, '.oct');
const terminalCapabilitiesPath = path.join(octDir, 'terminal-capabilities.json');

console.log(`one-click-tools: Downloading ${archiveName} from GitHub...`);

function detectImageIconSupport() {
    const termProgram = (process.env.TERM_PROGRAM || '').toLowerCase();
    const term = (process.env.TERM || '').toLowerCase();
    const terminalEmulator = (process.env.TERMINAL_EMULATOR || '').toLowerCase();
    const termFeatures = (process.env.TERM_FEATURES || '').toLowerCase();

    const isKnownImageTerminal =
        termProgram === 'iterm.app' ||
        termProgram === 'wezterm' ||
        termProgram === 'ghostty' ||
        terminalEmulator === 'kitty' ||
        term.includes('kitty');

    const hasMultiplexer =
        !!process.env.TMUX ||
        !!process.env.STY ||
        !!process.env.ZELLIJ ||
        !!process.env.CMUX ||
        term.includes('tmux') ||
        term.includes('screen') ||
        term.includes('cmux');

    const chafaAvailable = (() => {
        try {
            execSync('chafa --version', { stdio: 'ignore' });
            return true;
        } catch {
            return false;
        }
    })();

    const sixelSupported =
        term.includes('sixel') ||
        termFeatures.includes('sixel') ||
        process.env.XTERM_SIXEL === '1';

    const imageIconsSupported = isKnownImageTerminal && !hasMultiplexer;
    const bestRenderer = imageIconsSupported ? 'native_image' : 'text';

    return {
        image_icons_supported: imageIconsSupported,
        best_renderer: bestRenderer,
        chafa_available: chafaAvailable,
        sixel_supported: sixelSupported,
        detected_terminal: termProgram || term || 'unknown',
        has_multiplexer: hasMultiplexer,
        detected_at: new Date().toISOString(),
    };
}

function writeTerminalCapabilities() {
    try {
        fs.mkdirSync(octDir, { recursive: true });
        const capability = detectImageIconSupport();
        fs.writeFileSync(terminalCapabilitiesPath, JSON.stringify(capability, null, 2), 'utf8');
        console.log(
            `one-click-tools: terminal image icon support = ${capability.image_icons_supported ? 'enabled' : 'disabled'} (${capability.detected_terminal})`
        );
    } catch (err) {
        console.warn(`one-click-tools: Failed to write terminal capability file: ${err.message}`);
    }
}

function download(url, dest) {
    return new Promise((resolve, reject) => {
        https.get(url, (response) => {
            if (response.statusCode === 302 || response.statusCode === 301) {
                download(response.headers.location, dest).then(resolve).catch(reject);
                return;
            }
            if (response.statusCode !== 200) {
                reject(new Error(`Failed to download: HTTP ${response.statusCode} for ${url}`));
                return;
            }
            const file = fs.createWriteStream(dest);
            response.pipe(file);
            file.on('finish', () => {
                file.close(resolve);
            });
            file.on('error', (err) => {
                fs.unlink(dest, () => {});
                reject(err);
            });
        }).on('error', (err) => {
            reject(err);
        });
    });
}

async function install() {
    const archivePath = path.join(os.tmpdir(), archiveName);
    try {
        writeTerminalCapabilities();
        console.log(`one-click-tools: Downloading from ${downloadUrl}...`);
        await download(downloadUrl, archivePath);
        console.log('one-click-tools: Extracting binary...');
        
        if (archiveName.endsWith('.zip')) {
            // Simple zip extraction for windows (assuming powershell available)
            execSync(`powershell -Command "Expand-Archive -Path '${archivePath}' -DestinationPath '${binDir}' -Force"`);
        } else {
            // tar.gz extraction for mac/linux
            execSync(`tar -xzf "${archivePath}" -C "${binDir}"`);
        }
        
        // Verify multiple possible locations in case tarball structure is different
        let foundPath = null;
        if (fs.existsSync(targetPath)) {
            foundPath = targetPath;
        } else {
            // Check if binary is nested in a directory inside the archive
            const files = fs.readdirSync(binDir);
            for (const file of files) {
                const fullPath = path.join(binDir, file);
                if (fs.statSync(fullPath).isDirectory()) {
                    const nestedPath = path.join(fullPath, binName);
                    if (fs.existsSync(nestedPath)) {
                        foundPath = nestedPath;
                        // Move it to the expected targetPath
                        fs.renameSync(nestedPath, targetPath);
                        foundPath = targetPath;
                        break;
                    }
                }
            }
        }

        if (foundPath && fs.existsSync(foundPath)) {
            fs.chmodSync(foundPath, 0o755);
            console.log('one-click-tools: Installation successful.');
        } else {
            throw new Error(`Binary '${binName}' not found in ${binDir} after extraction.`);
        }
    } catch (err) {
        console.error(`one-click-tools: Installation failed: ${err.message}`);
        console.log('one-click-tools: Falling back to source build (requires Go)...');
        try {
            execSync('npm run build', { stdio: 'inherit', cwd: path.join(__dirname, '..') });
            const builtBin = path.join(__dirname, '..', binName);
            if (fs.existsSync(builtBin)) {
                if (!fs.existsSync(binDir)) fs.mkdirSync(binDir, { recursive: true });
                fs.renameSync(builtBin, targetPath);
                fs.chmodSync(targetPath, 0o755);
                console.log('one-click-tools: Source build and installation successful.');
            } else {
                throw new Error('Source build did not produce the expected binary.');
            }
        } catch (buildErr) {
            console.error(`one-click-tools: Fallback build failed: ${buildErr.message}`);
            console.error('one-click-tools: Please ensure Go is installed or check your internet connection.');
        }
    } finally {
        if (fs.existsSync(archivePath)) {
            try { fs.unlinkSync(archivePath); } catch (e) {}
        }
    }
}

install();
