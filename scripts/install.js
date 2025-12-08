#!/usr/bin/env node

/**
 * CLASP Installation Script
 * Downloads the pre-built Go binary for the current platform
 */

const https = require('https');
const http = require('http');
const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
const os = require('os');

const VERSION = '0.48.5';
const REPO = 'jedarden/CLASP';
const BINARY_NAME = 'clasp';

// Platform mappings
const PLATFORM_MAP = {
  darwin: 'darwin',
  linux: 'linux',
  win32: 'windows'
};

const ARCH_MAP = {
  x64: 'amd64',
  arm64: 'arm64'
};

function getPlatformInfo() {
  const platform = PLATFORM_MAP[os.platform()];
  const arch = ARCH_MAP[os.arch()];

  if (!platform) {
    throw new Error(`Unsupported platform: ${os.platform()}`);
  }
  if (!arch) {
    throw new Error(`Unsupported architecture: ${os.arch()}`);
  }

  return { platform, arch };
}

function getBinaryName(platform) {
  return platform === 'windows' ? `${BINARY_NAME}.exe` : BINARY_NAME;
}

function getDownloadUrl(platform, arch) {
  const ext = platform === 'windows' ? '.exe' : '';
  const filename = `clasp-${platform}-${arch}${ext}`;
  return `https://github.com/${REPO}/releases/download/v${VERSION}/${filename}`;
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    const protocol = url.startsWith('https') ? https : http;

    const request = protocol.get(url, (response) => {
      // Handle redirects
      if (response.statusCode === 301 || response.statusCode === 302) {
        file.close();
        fs.unlinkSync(dest);
        return download(response.headers.location, dest)
          .then(resolve)
          .catch(reject);
      }

      if (response.statusCode !== 200) {
        file.close();
        fs.unlinkSync(dest);
        reject(new Error(`Failed to download: HTTP ${response.statusCode}`));
        return;
      }

      response.pipe(file);

      file.on('finish', () => {
        file.close();
        resolve();
      });
    });

    request.on('error', (err) => {
      file.close();
      fs.unlink(dest, () => {}); // Delete partial file
      reject(err);
    });
  });
}

async function buildFromSource() {
  console.log('[CLASP] Pre-built binary not available, building from source...');

  // Check if Go is installed
  try {
    execSync('go version', { stdio: 'pipe' });
  } catch {
    throw new Error(
      'Go is not installed. Please install Go from https://go.dev/dl/ or download a pre-built binary from ' +
      `https://github.com/${REPO}/releases`
    );
  }

  const packageDir = path.dirname(__dirname);
  const binDir = path.join(packageDir, 'bin');

  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  console.log('[CLASP] Building binary...');
  execSync(`go build -o ${path.join(binDir, BINARY_NAME)} ./cmd/clasp`, {
    cwd: packageDir,
    stdio: 'inherit'
  });

  console.log('[CLASP] Build complete!');
}

async function main() {
  console.log(`[CLASP] Installing CLASP v${VERSION}...`);

  const { platform, arch } = getPlatformInfo();
  const binaryName = getBinaryName(platform);
  const binDir = path.join(path.dirname(__dirname), 'bin');
  const binaryPath = path.join(binDir, binaryName);

  // Create bin directory if it doesn't exist
  if (!fs.existsSync(binDir)) {
    fs.mkdirSync(binDir, { recursive: true });
  }

  // Check if binary already exists and is correct version
  if (fs.existsSync(binaryPath)) {
    try {
      const versionOutput = execSync(`"${binaryPath}" -version`, { encoding: 'utf8' });
      if (versionOutput.includes(VERSION)) {
        console.log(`[CLASP] CLASP v${VERSION} is already installed.`);
        return;
      }
    } catch {
      // Version check failed, proceed with download
    }
  }

  const url = getDownloadUrl(platform, arch);
  console.log(`[CLASP] Downloading from ${url}...`);

  try {
    await download(url, binaryPath);

    // Make executable on Unix
    if (platform !== 'windows') {
      fs.chmodSync(binaryPath, 0o755);
    }

    console.log(`[CLASP] Successfully installed to ${binaryPath}`);
  } catch (err) {
    console.warn(`[CLASP] Download failed: ${err.message}`);

    // Try building from source
    try {
      await buildFromSource();
    } catch (buildErr) {
      console.error(`[CLASP] Installation failed: ${buildErr.message}`);
      console.error('[CLASP] Please ensure Go is installed or download the binary manually from:');
      console.error(`[CLASP] https://github.com/${REPO}/releases`);
      process.exit(1);
    }
  }
}

main().catch((err) => {
  console.error(`[CLASP] Installation error: ${err.message}`);
  process.exit(1);
});
