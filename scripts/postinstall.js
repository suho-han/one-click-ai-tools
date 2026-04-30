const os = require('os');
const fs = require('fs');
const path = require('path');
const https = require('https');
const { execSync } = require('child_process');

const version = 'v0.3.0'; // Should ideally be dynamic or matched to package.json
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

console.log(`one-click-tools: Downloading ${archiveName} from GitHub...`);

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
