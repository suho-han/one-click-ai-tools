#!/bin/bash

# AI CLI Tools Update Script
# Updates: claude-code, codex-cli, gemini-cli, copilot-cli

set -u

# ANSI Color Codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== AI CLI Tools Update Start ===${NC}"

TOOLS=("claude-code" "codex" "gemini-cli" "copilot-cli")
NPM_PACKAGES=(
    "@anthropic-ai/claude-code"
    "@openai/codex"
    "@google/gemini-cli"
    "@github/copilot"
)

SUCCESS_ITEMS=()
FAILED_ITEMS=()

record_success() {
    SUCCESS_ITEMS+=("$1")
}

record_failure() {
    FAILED_ITEMS+=("$1")
    echo -e "${RED}Failed: $1${NC}"
}

warn_missing_manager_and_exit() {
    local manager="$1"
    echo -e "${YELLOW}Required package manager not found (${manager}). Please install it and rerun.${NC}"
    exit 1
}

summarize_results() {
    echo -e "${BLUE}=== Summary ===${NC}"
    echo -e "${GREEN}Succeeded: ${#SUCCESS_ITEMS[@]}${NC}"
    for item in "${SUCCESS_ITEMS[@]}"; do
        echo -e "  ${GREEN}- ${item}${NC}"
    done

    if [[ ${#FAILED_ITEMS[@]} -gt 0 ]]; then
        echo -e "${RED}Failed: ${#FAILED_ITEMS[@]}${NC}"
        for item in "${FAILED_ITEMS[@]}"; do
            echo -e "  ${RED}- ${item}${NC}"
        done
    else
        echo -e "${GREEN}Failed: 0${NC}"
    fi
}

run_npm_with_sudo_retry() {
    local action="$1"
    local pkg="$2"
    local output

    output="$(npm "$action" -g "$pkg" 2>&1)"
    local rc=$?

    if [[ $rc -eq 0 ]]; then
        [[ -n "$output" ]] && echo "$output"
        return 0
    fi

    [[ -n "$output" ]] && echo "$output"
    if echo "$output" | grep -qiE "EACCES|permission denied"; then
        echo -e "${YELLOW}Permission issue detected for ${pkg}. Retrying with sudo...${NC}"
        sudo npm "$action" -g "$pkg"
        return $?
    fi

    return $rc
}

update_macos() {
    # Check for Homebrew
    if ! command -v brew &> /dev/null; then
        warn_missing_manager_and_exit "brew"
    fi

    echo -e "${YELLOW}Detected macOS. Updating Homebrew formulae...${NC}"
    if ! brew update; then
        echo -e "${YELLOW}Warning: brew update failed. Continuing with tool-level install/upgrade attempts.${NC}"
    fi

    for tool in "${TOOLS[@]}"; do
        echo -e "${BLUE}Checking update for: ${tool}...${NC}"
        if brew list "$tool" &> /dev/null; then
            echo -e "${YELLOW}Upgrading ${tool}...${NC}"
            if brew upgrade "$tool"; then
                record_success "${tool} (brew upgrade)"
            else
                record_failure "${tool} (brew upgrade)"
            fi
        else
            echo -e "${YELLOW}${tool} is not installed via Homebrew. Installing...${NC}"
            if brew install "$tool"; then
                record_success "${tool} (brew install)"
            else
                record_failure "${tool} (brew install)"
            fi
        fi
    done
}

update_ubuntu() {
    if ! command -v npm &> /dev/null; then
        warn_missing_manager_and_exit "npm"
    fi

    echo -e "${YELLOW}Detected Ubuntu. Updating npm global packages...${NC}"

    for i in "${!TOOLS[@]}"; do
        tool="${TOOLS[$i]}"
        pkg="${NPM_PACKAGES[$i]}"

        echo -e "${BLUE}Checking update for: ${tool} (${pkg})...${NC}"
        if npm list -g --depth=0 "$pkg" &> /dev/null; then
            echo -e "${YELLOW}Upgrading ${pkg}...${NC}"
            if run_npm_with_sudo_retry "update" "$pkg"; then
                record_success "${tool} (${pkg}, npm update -g)"
            else
                record_failure "${tool} (${pkg}, npm update -g)"
            fi
        else
            echo -e "${YELLOW}${pkg} is not installed globally via npm. Installing...${NC}"
            if run_npm_with_sudo_retry "install" "$pkg"; then
                record_success "${tool} (${pkg}, npm install -g)"
            else
                record_failure "${tool} (${pkg}, npm install -g)"
            fi
        fi
    done
}

OS_NAME="$(uname -s)"

case "$OS_NAME" in
    Darwin)
        update_macos
        ;;
    Linux)
        if [[ -f /etc/os-release ]]; then
            . /etc/os-release
            if [[ "$ID" == "ubuntu" ]]; then
                update_ubuntu
            else
                echo -e "${YELLOW}Unsupported Linux distribution: ${ID}. This script currently supports Ubuntu only.${NC}"
                exit 1
            fi
        else
            echo -e "${YELLOW}Cannot detect Linux distribution (/etc/os-release not found).${NC}"
            exit 1
        fi
        ;;
    *)
        echo -e "${YELLOW}Unsupported OS: ${OS_NAME}. This script supports macOS and Ubuntu only.${NC}"
        exit 1
        ;;
esac

# Copilot CLI extra check (it has its own update command)
if command -v copilot &> /dev/null; then
    echo -e "${BLUE}Running copilot update check...${NC}"
    if copilot update; then
        record_success "copilot self-update"
    else
        echo -e "${YELLOW}Warning: copilot self-update failed.${NC}"
        record_failure "copilot self-update"
    fi
fi

summarize_results
echo -e "${GREEN}=== AI CLI Tools Update Complete! ===${NC}"
