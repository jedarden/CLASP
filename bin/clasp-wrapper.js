#!/usr/bin/env node

/**
 * CLASP Wrapper Script
 * Executes the Go binary with all passed arguments
 */

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

const BINARY_NAME = os.platform() === 'win32' ? 'clasp.exe' : 'clasp';
const binaryPath = path.join(__dirname, BINARY_NAME);

// Check if binary exists
if (!fs.existsSync(binaryPath)) {
  console.error('[CLASP] Binary not found. Running install script...');
  require('../scripts/install.js');
}

// Spawn the Go binary with all arguments
const args = process.argv.slice(2);
const child = spawn(binaryPath, args, {
  stdio: 'inherit',
  env: process.env
});

child.on('error', (err) => {
  if (err.code === 'ENOENT') {
    console.error('[CLASP] Binary not found. Please run: npm run postinstall');
  } else {
    console.error(`[CLASP] Error: ${err.message}`);
  }
  process.exit(1);
});

child.on('close', (code) => {
  process.exit(code || 0);
});
