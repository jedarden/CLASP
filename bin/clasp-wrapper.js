#!/usr/bin/env node

/**
 * CLASP Wrapper Script
 * Executes the Go binary with all passed arguments
 *
 * Handles proper signal forwarding and process cleanup to prevent zombie processes.
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

// Track if we're already cleaning up to prevent double-exit
let isCleaningUp = false;

// Spawn the Go binary with all arguments
const args = process.argv.slice(2);
const child = spawn(binaryPath, args, {
  stdio: 'inherit',
  env: process.env,
  // Detach the child on Windows to prevent zombie issues
  detached: os.platform() === 'win32'
});

// Store the PID for cleanup
const childPid = child.pid;

// Forward signals to child process
const forwardSignal = (signal) => {
  if (child && !child.killed && childPid) {
    try {
      // On Unix, send signal to the child process
      process.kill(childPid, signal);
    } catch (err) {
      // Process may have already exited, ignore errors
    }
  }
};

// Handle common termination signals
['SIGINT', 'SIGTERM', 'SIGHUP'].forEach((signal) => {
  process.on(signal, () => {
    if (isCleaningUp) return;
    isCleaningUp = true;
    forwardSignal(signal);
  });
});

// Handle parent process exit to ensure child cleanup
process.on('exit', () => {
  if (child && !child.killed && childPid) {
    try {
      // Send SIGTERM on exit to ensure child process terminates
      process.kill(childPid, 'SIGTERM');
    } catch (err) {
      // Process may have already exited, ignore errors
    }
  }
});

// Handle uncaught exceptions - cleanup and exit
process.on('uncaughtException', (err) => {
  console.error(`[CLASP] Uncaught exception: ${err.message}`);
  if (child && !child.killed && childPid) {
    try {
      process.kill(childPid, 'SIGTERM');
    } catch (e) {
      // Ignore
    }
  }
  process.exit(1);
});

child.on('error', (err) => {
  if (err.code === 'ENOENT') {
    console.error('[CLASP] Binary not found. Please run: npm run postinstall');
  } else {
    console.error(`[CLASP] Error: ${err.message}`);
  }
  process.exit(1);
});

child.on('close', (code, signal) => {
  // Child process has fully closed, all stdio streams are closed
  // Exit with the same code as the child
  if (isCleaningUp) return;
  isCleaningUp = true;
  process.exit(code || 0);
});

child.on('exit', (code, signal) => {
  // Child process has exited but stdio may still be open
  // The 'close' event will handle final cleanup
});
