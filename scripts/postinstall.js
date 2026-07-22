const os = require('os');
const fs = require('fs');
const path = require('path');
const https = require('https');
const readline = require('readline');
const { execFileSync, execSync } = require('child_process');

const packageJson = require(path.join(__dirname, '..', 'package.json'));
const version = `v${packageJson.version}`;
const repo = 'suho-han/one-click-ai-tools';

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
const archiveName = `one-click-ai-tools_${platform}_${arch}${platform === 'windows' ? '.zip' : '.tar.gz'}`;
const downloadUrl = `https://github.com/${repo}/releases/download/${version}/${archiveName}`;

const binDir = path.join(__dirname, '..', 'bin');
if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
}

const targetPath = path.join(binDir, binName);
const homeDir = os.homedir();
const octDir = path.join(homeDir, '.oct');
const terminalCapabilitiesPath = path.join(octDir, 'terminal-capabilities.json');

console.log(`one-click-ai-tools: Downloading ${archiveName} from GitHub...`);

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
            `one-click-ai-tools: terminal image icon support = ${capability.image_icons_supported ? 'enabled' : 'disabled'} (${capability.detected_terminal})`
        );
    } catch (err) {
        console.warn(`one-click-ai-tools: Failed to write terminal capability file: ${err.message}`);
    }
}

function sessionRefreshConfigPath() {
    return path.join(octDir, 'config.yaml');
}

function setYamlScalar(content, key, value) {
    const serialized = typeof value === 'string' ? value : String(value);
    const pattern = new RegExp(`^${key}:.*$`, 'm');
    const line = `${key}: ${serialized}`;
    if (pattern.test(content)) {
        return content.replace(pattern, line);
    }
    if (content.trim() === '') {
        return `${line}\n`;
    }
    return `${content.replace(/\s*$/, '')}\n${line}\n`;
}

function writeSessionRefreshConfig(enabled, interval, hour) {
    fs.mkdirSync(octDir, { recursive: true });
    const configPath = sessionRefreshConfigPath();
    let content = '';
    if (fs.existsSync(configPath)) {
        content = fs.readFileSync(configPath, 'utf8');
    }
    content = setYamlScalar(content, 'session_refresh_enabled', enabled);
    content = setYamlScalar(content, 'session_refresh_interval', interval);
    content = setYamlScalar(content, 'session_refresh_hour', hour);
    fs.writeFileSync(configPath, content, 'utf8');
    return configPath;
}

function isInteractiveInstall() {
    return Boolean(process.stdin.isTTY && process.stdout.isTTY && !process.env.CI);
}

function askQuestion(prompt) {
    return new Promise((resolve) => {
        const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
        rl.question(prompt, (answer) => {
            rl.close();
            resolve((answer || '').trim());
        });
    });
}

function truthyEnv(value) {
    return ['1', 'true', 'yes', 'on'].includes((value || '').trim().toLowerCase());
}

function falsyEnv(value) {
    return ['0', 'false', 'no', 'off'].includes((value || '').trim().toLowerCase());
}

function isPathInside(candidate, parent) {
    const relative = path.relative(path.resolve(parent), path.resolve(candidate));
    return relative === '' || (!!relative && !relative.startsWith('..') && !path.isAbsolute(relative));
}

function isGlobalInstall() {
    if (truthyEnv(process.env.npm_config_global) || truthyEnv(process.env.NPM_CONFIG_GLOBAL)) {
        return true;
    }

    const prefix = process.env.npm_config_prefix || process.env.NPM_CONFIG_PREFIX;
    if (!prefix) {
        return false;
    }

    const packageRoot = path.resolve(__dirname, '..');
    return [
        path.join(prefix, 'lib', 'node_modules'),
        path.join(prefix, 'node_modules'),
        path.join(prefix, 'global'),
    ].some((globalRoot) => isPathInside(packageRoot, globalRoot));
}

function shouldInstallCompletion() {
    const envChoice = process.env.OCT_INSTALL_COMPLETION;
    if (falsyEnv(envChoice)) {
        return false;
    }
    if (truthyEnv(envChoice)) {
        return true;
    }
    return isGlobalInstall();
}

function detectUserShell() {
    const shellName = path.basename(process.env.SHELL || '').toLowerCase();
    if (shellName.includes('zsh')) {
        return 'zsh';
    }
    if (shellName.includes('bash')) {
        return 'bash';
    }
    if (shellName.includes('fish')) {
        return 'fish';
    }
    return '';
}

function appendManagedBlock(filePath, beginMarker, endMarker, body) {
    fs.mkdirSync(path.dirname(filePath), { recursive: true });
    let content = '';
    if (fs.existsSync(filePath)) {
        content = fs.readFileSync(filePath, 'utf8');
    }
    if (content.includes(beginMarker)) {
        return false;
    }

    const prefix = content === '' || content.endsWith('\n') ? content : `${content}\n`;
    fs.writeFileSync(filePath, `${prefix}${beginMarker}\n${body}\n${endMarker}\n`, 'utf8');
    return true;
}

function installShellCompletion(binaryPath, shell) {
    const completion = execFileSync(binaryPath, ['completion', shell], {
        encoding: 'utf8',
        stdio: ['ignore', 'pipe', 'pipe'],
    });

    if (shell === 'fish') {
        const fishCompletionPath = path.join(homeDir, '.config', 'fish', 'completions', 'oct.fish');
        fs.mkdirSync(path.dirname(fishCompletionPath), { recursive: true });
        fs.writeFileSync(fishCompletionPath, completion, 'utf8');
        return 'fish completion installed.';
    }

    const completionRoot = path.join(octDir, 'completions', shell);
    fs.mkdirSync(completionRoot, { recursive: true });

    if (shell === 'zsh') {
        const completionPath = path.join(completionRoot, '_oct');
        fs.writeFileSync(completionPath, completion, 'utf8');
        const zshrcPath = path.join(homeDir, '.zshrc');
        const begin = '# >>> one-click-ai-tools completion >>>';
        const end = '# <<< one-click-ai-tools completion <<<';
        const body = [
            `fpath=("${completionRoot}" $fpath)`,
            'autoload -Uz compinit',
            'compinit',
        ].join('\n');
        const updated = appendManagedBlock(zshrcPath, begin, end, body);
        return updated ? 'zsh completion installed. Restart your shell or run: source ~/.zshrc' : 'zsh completion already configured.';
    }

    if (shell === 'bash') {
        const completionPath = path.join(completionRoot, 'oct.bash');
        fs.writeFileSync(completionPath, completion, 'utf8');
        const bashrcPath = path.join(homeDir, '.bashrc');
        const begin = '# >>> one-click-ai-tools completion >>>';
        const end = '# <<< one-click-ai-tools completion <<<';
        const body = `[ -f "${completionPath}" ] && source "${completionPath}"`;
        const updated = appendManagedBlock(bashrcPath, begin, end, body);
        return updated ? 'bash completion installed. Restart your shell or run: source ~/.bashrc' : 'bash completion already configured.';
    }

    return '';
}

function maybeInstallShellCompletion(binaryPath) {
    if (!shouldInstallCompletion()) {
        return;
    }

    const shell = detectUserShell();
    if (!shell) {
        console.log('one-click-ai-tools: shell completion skipped (unsupported or unknown $SHELL).');
        return;
    }

    try {
        const message = installShellCompletion(binaryPath, shell);
        if (message) {
            console.log(`one-click-ai-tools: ${message}`);
        }
    } catch (err) {
        console.warn(`one-click-ai-tools: Failed to install shell completion automatically: ${err.message}`);
        console.warn(`one-click-ai-tools: You can install it later with: oct completion ${shell}`);
    }
}

async function maybeConfigureSessionRefresh(binaryPath) {
    const envChoice = (process.env.OCT_INSTALL_ENABLE_SESSION_REFRESH || '').trim().toLowerCase();
    if (envChoice === '0' || envChoice === 'false' || envChoice === 'no') {
        writeSessionRefreshConfig(false, 'daily', 9);
        console.log('one-click-ai-tools: session-refresh left disabled in config.');
        return;
    }

    let enable = false;
    if (envChoice === '1' || envChoice === 'true' || envChoice === 'yes') {
        enable = true;
    } else if (isInteractiveInstall()) {
        const answer = await askQuestion('Enable periodic token-free session refresh? [y/N]: ');
        enable = answer === 'y' || answer === 'yes';
    }

    const interval = 'daily';
    const hour = 9;
    const configPath = writeSessionRefreshConfig(enable, interval, hour);
    if (!enable) {
        console.log(`one-click-ai-tools: session-refresh defaults saved to ${configPath} (disabled, daily 09:00).`);
        return;
    }

    try {
        execSync(`"${binaryPath}" schedule enable --task session-refresh --interval ${interval} --hour ${hour}`, {
            stdio: 'inherit',
            cwd: homeDir,
        });
        console.log(`one-click-ai-tools: session-refresh enabled (${interval}, ${hour}:00).`);
    } catch (err) {
        console.warn(`one-click-ai-tools: Failed to enable session-refresh schedule automatically: ${err.message}`);
        console.warn('one-click-ai-tools: You can enable it later with: oct schedule config --enabled --interval daily --hour 9');
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
        console.log(`one-click-ai-tools: Downloading from ${downloadUrl}...`);
        await download(downloadUrl, archivePath);
        console.log('one-click-ai-tools: Extracting binary...');
        
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
            console.log('one-click-ai-tools: Installation successful.');
            maybeInstallShellCompletion(foundPath);
            await maybeConfigureSessionRefresh(foundPath);
        } else {
            throw new Error(`Binary '${binName}' not found in ${binDir} after extraction.`);
        }
    } catch (err) {
        console.error(`one-click-ai-tools: Installation failed: ${err.message}`);
        console.log('one-click-ai-tools: Falling back to source build (requires Go)...');
        try {
            execSync('npm run build', { stdio: 'inherit', cwd: path.join(__dirname, '..') });
            const builtBin = path.join(__dirname, '..', binName);
            if (fs.existsSync(builtBin)) {
                if (!fs.existsSync(binDir)) fs.mkdirSync(binDir, { recursive: true });
                fs.renameSync(builtBin, targetPath);
                fs.chmodSync(targetPath, 0o755);
                console.log('one-click-ai-tools: Source build and installation successful.');
                maybeInstallShellCompletion(targetPath);
                await maybeConfigureSessionRefresh(targetPath);
            } else {
                throw new Error('Source build did not produce the expected binary.');
            }
        } catch (buildErr) {
            console.error(`one-click-ai-tools: Fallback build failed: ${buildErr.message}`);
            console.error('one-click-ai-tools: Please ensure Go is installed or check your internet connection.');
        }
    } finally {
        if (fs.existsSync(archivePath)) {
            try { fs.unlinkSync(archivePath); } catch (e) {}
        }
    }
}

install();
