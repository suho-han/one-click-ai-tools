#!/bin/bash

# Load environment variables from .env if it exists
if [ -f .env ]; then
  echo "Loading environment variables from .env..."
  export $(grep -v '^#' .env | xargs)
fi

# Flag to track overall success
SUCCESS=true

# Attempt pnpm publish
if command -v pnpm &> /dev/null; then
  echo "--- Starting pnpm publish ---"
  if pnpm publish --no-git-checks; then
    echo "pnpm publish successful!"
  else
    echo "pnpm publish failed!"
    SUCCESS=false
  fi
else
  echo "pnpm not found, skipping..."
fi

echo "" # Add a newline for readability

# Attempt npm publish
if command -v npm &> /dev/null; then
  echo "--- Starting npm publish ---"
  if npm publish; then
    echo "npm publish successful!"
  else
    echo "npm publish failed!"
    SUCCESS=false
  fi
else
  echo "npm not found, skipping..."
fi

if [ "$SUCCESS" = true ]; then
  echo ""
  echo "All publish attempts completed successfully!"
else
  echo ""
  echo "One or more publish attempts failed. Please check the logs above."
  exit 1
fi
