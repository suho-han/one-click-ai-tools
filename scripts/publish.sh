#!/bin/bash

# Exit on error
set -e

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
  echo "Standard-version failed. Make sure your working directory is clean."
  exit 1
fi

# 3. Push to GitHub
echo ""
echo "--- Step 2: Pushing to GitHub with tags ---"
git push --follow-tags origin main

# 4. Publishing
SUCCESS=true

# Attempt pnpm publish
if command -v pnpm &> /dev/null; then
  echo ""
  echo "--- Step 3 (A): Starting pnpm publish ---"
  if pnpm publish --no-git-checks; then
    echo "pnpm publish successful!"
  else
    echo "pnpm publish failed!"
    SUCCESS=false
  fi
else
  echo "pnpm not found, skipping..."
fi

# Attempt npm publish
if command -v npm &> /dev/null; then
  echo ""
  echo "--- Step 3 (B): Starting npm publish ---"
  if npm publish; then
    echo "npm publish successful!"
  else
    echo "npm publish failed!"
    SUCCESS=false
  fi
else
  echo "npm not found, skipping..."
fi

# Final status
if [ "$SUCCESS" = true ]; then
  echo ""
  echo "=========================================="
  echo "✅ All steps completed successfully!"
  echo "1. Git Commit & Tag created"
  echo "2. Pushed to GitHub"
  echo "3. Published to Package Registries"
  echo "=========================================="
else
  echo ""
  echo "❌ One or more publish attempts failed. Please check the logs above."
  exit 1
fi
