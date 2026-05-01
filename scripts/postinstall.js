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
    const bestRenderer = imageIconsSupported ? 'native_image' : 'ansi_asset';

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
        const file = fs.createWriteStream(dest);
        https.get(url, (response) => {
            if (response.statusCode === 302 || response.statusCode === 301) {
                download(response.headers.location, dest).then(resolve).catch(reject);
                return;
            }
            if (response.statusCode !== 200) {
                reject(new Error(`Failed to download: HTTP ${response.statusCode}`));
                return;
            }
            response.pipe(file);
            file.on('finish', () => {
                file.close(resolve);
            });
        }).on('error', (err) => {
            fs.unlink(dest, () => {});
            reject(err);
        });
    });
}

async function install() {
    const archivePath = path.join(os.tmpdir(), archiveName);
    try {
        writeTerminalCapabilities();
        await download(downloadUrl, archivePath);
        console.log('one-click-tools: Extracting binary...');
        
        if (archiveName.endsWith('.zip')) {
            // Simple zip extraction for windows (assuming powershell available)
            execSync(`powershell -Command "Expand-Archive -Path '${archivePath}' -DestinationPath '${binDir}' -Force"`);
        } else {
            // tar.gz extraction for mac/linux
            execSync(`tar -xzf "${archivePath}" -C "${binDir}"`);
        }
        
        if (fs.existsSync(targetPath)) {
            fs.chmodSync(targetPath, 0755);
            console.log('one-click-tools: Installation successful.');
        } else {
            throw new Error('Binary not found after extraction');
        }
    } catch (err) {
        console.error(`one-click-tools: Installation failed: ${err.message}`);
        console.log('one-click-tools: Falling back to source build (requires Go)...');
        try {
            execSync('npm run build', { stdio: 'inherit', cwd: path.join(__dirname, '..') });
        } catch (buildErr) {
            console.error('one-click-tools: Fallback build failed.');
        }
    } finally {
        if (fs.existsSync(archivePath)) {
            fs.unlinkSync(archivePath);
        }
    }
}

install();
