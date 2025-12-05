#!/usr/bin/env node

/**
 * CLASP Prepack Script
 * Prepares the package for npm publish by removing binaries
 * (users will download their platform-specific binary on install)
 */

const fs = require('fs');
const path = require('path');

const binDir = path.join(path.dirname(__dirname), 'bin');

// List of files to keep in bin/
const KEEP_FILES = ['clasp-wrapper.js'];

console.log('[CLASP] Preparing package for npm...');

// Remove compiled binaries (they'll be downloaded on install)
if (fs.existsSync(binDir)) {
  const files = fs.readdirSync(binDir);
  for (const file of files) {
    if (!KEEP_FILES.includes(file)) {
      const filePath = path.join(binDir, file);
      fs.unlinkSync(filePath);
      console.log(`[CLASP] Removed ${file}`);
    }
  }
}

console.log('[CLASP] Package prepared successfully!');
