#!/bin/bash

set -euo pipefail

# 1. Load environment variables from .env if it exists
if [ -f .env ]; then
  echo "Loading environment variables from .env..."
  export $(grep -v '^#' .env | xargs)
fi

# 2. Release & Tagging (Standard Version)
echo "--- Step 1: Bumping version and tagging ---"
if npx standard-version; then
  echo "Version bump and tagging successful!"
else
  echo "standard-version failed. Make sure your working directory is clean."
  exit 1
fi

# 3. Push to GitHub
echo ""
echo "--- Step 2: Pushing to GitHub with tags ---"
git push --follow-tags origin main

# 4. Preflight checks
echo ""
echo "--- Step 3: Release integrity checks ---"
RELEASE_TAG="$(git describe --tags --abbrev=0)" bash scripts/verify-release-integrity.sh

# 5. npm publish only
echo ""
echo "--- Step 4: Starting npm publish ---"
npm publish

echo ""
echo "=========================================="
echo "✅ Release completed successfully"
echo "1. Git Commit & Tag created"
echo "2. Pushed to GitHub"
echo "3. Verified release integrity"
echo "4. Published to npm"
echo "=========================================="
