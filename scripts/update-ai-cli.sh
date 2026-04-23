#!/bin/bash

# one-click-tools (oct)
# OS-aware updater for developer AI CLI tools

set -u

# ANSI Color Codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color
LOG_FILE=""

# Preserve original stdout/stderr so we can always print log location to terminal.
exec 3>&1
exec 4>&2

# Tool configuration
TOOLS=("Claude Code" "OpenAI Codex" "Gemini CLI" "GitHub Copilot")
NPM_PACKAGES=(
    "@anthropic-ai/claude-code"
    "@openai/codex"
    "@google/gemini-cli"
    "@github/copilot"
)
BINARY_NAMES=(
    "claude"
    "codex"
    "gemini"
    "copilot"
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

print_log_location() {
    if [[ -n "${LOG_FILE}" ]]; then
        echo -e "${BLUE}Log saved to: ${LOG_FILE}${NC}" >&3
    fi
}

setup_logging_for_agent_update() {
    local log_dir="${HOME}/.oct/logs"
    local timestamp
    timestamp="$(date +%Y%m%d-%H%M%S)"
    mkdir -p "$log_dir"
    LOG_FILE="${log_dir}/agent-update-${timestamp}.log"

    # Disable ANSI color codes so log files remain plain text.
    GREEN=''
    BLUE=''
    YELLOW=''
    RED=''
    NC=''

    # Mirror all output to terminal and log file.
    exec > >(tee -a "$LOG_FILE")
    exec 2>&1

    trap print_log_location EXIT
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
    if echo "$output" | grep -qiE "EEXIST|File exists"; then
        return 42
    fi

    if echo "$output" | grep -qiE "EACCES|permission denied"; then
        echo -e "${YELLOW}Permission issue detected for ${pkg}. Retrying with sudo...${NC}"
        sudo npm "$action" -g "$pkg"
        return $?
    fi

    return $rc
}

run_npm_force_install_with_sudo_retry() {
    local pkg="$1"
    local output

    output="$(npm install -g --force "$pkg" 2>&1)"
    local rc=$?

    if [[ $rc -eq 0 ]]; then
        [[ -n "$output" ]] && echo "$output"
        return 0
    fi

    [[ -n "$output" ]] && echo "$output"
    if echo "$output" | grep -qiE "EACCES|permission denied"; then
        echo -e "${YELLOW}Permission issue detected for ${pkg}. Retrying forced install with sudo...${NC}"
        sudo npm install -g --force "$pkg"
        return $?
    fi

    return $rc
}

binary_exists() {
    local bin_name="$1"
    command -v "$bin_name" &> /dev/null
}

try_brew_update_for_tool() {
    local tool_name="$1"
    local pkg="$2"
    local candidates=()
    local candidate

    if ! command -v brew &> /dev/null; then
        return 2
    fi

    case "$pkg" in
        "@anthropic-ai/claude-code")
            candidates=("claude-code")
            ;;
        "@openai/codex")
            candidates=("codex")
            ;;
        "@google/gemini-cli")
            candidates=("gemini-cli" "gemini")
            ;;
        "@github/copilot")
            candidates=("copilot")
            ;;
        *)
            return 2
            ;;
    esac

    for candidate in "${candidates[@]}"; do
        if brew list --formula "$candidate" &> /dev/null; then
            echo -e "${YELLOW}${tool_name} appears to be Homebrew formula-managed (${candidate}). Running brew upgrade ${candidate}...${NC}"
            if brew upgrade "$candidate"; then
                record_success "${tool_name} (${pkg}, brew formula upgrade: ${candidate})"
                return 0
            fi
            return 1
        fi

        if brew list --cask "$candidate" &> /dev/null; then
            echo -e "${YELLOW}${tool_name} appears to be Homebrew cask-managed (${candidate}). Running brew upgrade --cask ${candidate}...${NC}"
            if brew upgrade --cask "$candidate"; then
                record_success "${tool_name} (${pkg}, brew cask upgrade: ${candidate})"
                return 0
            fi
            return 1
        fi
    done

    return 2
}

try_non_npm_update() {
    local tool_name="$1"
    local pkg="$2"
    local bin_name="$3"

    if [[ "$pkg" == "@github/copilot" ]]; then
        echo -e "${YELLOW}${tool_name} non-npm install detected. Forcing npm install to align with npm version...${NC}"
        if run_npm_force_install_with_sudo_retry "$pkg"; then
            record_success "${tool_name} (${pkg}, npm install -g --force to align version)"
            return 0
        fi
        echo -e "${YELLOW}Warning: npm force install for ${tool_name} failed. Falling back to non-npm update path.${NC}"
    fi

    if try_brew_update_for_tool "$tool_name" "$pkg"; then
        return 0
    fi

    local brew_rc=$?
    if [[ $brew_rc -eq 1 ]]; then
        return 1
    fi

    if [[ "$pkg" == "@github/copilot" ]] && command -v "$bin_name" &> /dev/null; then
        echo -e "${YELLOW}${tool_name} appears to be self-update capable. Running ${bin_name} update...${NC}"
        if "$bin_name" update; then
            record_success "${tool_name} (${pkg}, self-update)"
            return 0
        fi
        return 1
    fi

    return 2
}

update_macos() {
    if ! command -v brew &> /dev/null; then
        warn_missing_manager_and_exit "brew"
    fi

    echo -e "${YELLOW}Detected macOS. Updating Homebrew formulae...${NC}"
    if ! brew update; then
        echo -e "${YELLOW}Warning: brew update failed. Continuing with tool-level install/upgrade attempts.${NC}"
    fi
    echo -e "${YELLOW}Upgrading outdated Homebrew formulae/casks...${NC}"
    if brew upgrade; then
        record_success "Homebrew packages (brew upgrade)"
    else
        echo -e "${YELLOW}Warning: brew upgrade failed. Continuing with tool-level install/upgrade attempts.${NC}"
    fi

    for i in "${!TOOLS[@]}"; do
        tool_name="${TOOLS[$i]}"
        pkg="${NPM_PACKAGES[$i]}"
        bin_name="${BINARY_NAMES[$i]}"
        echo -e "${BLUE}Checking update for: ${tool_name} (${pkg})...${NC}"
        
        if npm list -g --depth=0 "$pkg" &> /dev/null; then
            echo -e "${YELLOW}Upgrading ${pkg}...${NC}"
            if run_npm_with_sudo_retry "update" "$pkg"; then
                record_success "${tool_name} (${pkg}, npm update -g)"
            else
                record_failure "${tool_name} (${pkg}, npm update -g)"
            fi
        elif binary_exists "$bin_name"; then
            echo -e "${YELLOW}${bin_name} command already exists but ${pkg} is not installed globally via npm. Trying non-npm update path...${NC}"
            if try_non_npm_update "$tool_name" "$pkg" "$bin_name"; then
                :
            else
                rc=$?
                if [[ $rc -eq 2 ]]; then
                    record_failure "${tool_name} (${pkg}, non-npm install detected but update method not found)"
                else
                    record_failure "${tool_name} (${pkg}, non-npm update failed)"
                fi
            fi
        else
            echo -e "${YELLOW}${pkg} is not installed globally via npm. Installing...${NC}"
            if run_npm_with_sudo_retry "install" "$pkg"; then
                record_success "${tool_name} (${pkg}, npm install -g)"
            elif [[ $? -eq 42 ]]; then
                echo -e "${YELLOW}Binary collision detected while installing ${pkg}. An existing executable already occupies the command path.${NC}"
                record_success "${tool_name} (${pkg}, install skipped due to existing binary)"
            else
                record_failure "${tool_name} (${pkg}, npm install -g)"
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
        tool_name="${TOOLS[$i]}"
        pkg="${NPM_PACKAGES[$i]}"
        bin_name="${BINARY_NAMES[$i]}"

        echo -e "${BLUE}Checking update for: ${tool_name} (${pkg})...${NC}"
        if npm list -g --depth=0 "$pkg" &> /dev/null; then
            echo -e "${YELLOW}Upgrading ${pkg}...${NC}"
            if run_npm_with_sudo_retry "update" "$pkg"; then
                record_success "${tool_name} (${pkg}, npm update -g)"
            else
                record_failure "${tool_name} (${pkg}, npm update -g)"
            fi
        elif binary_exists "$bin_name"; then
            echo -e "${YELLOW}${bin_name} command already exists but ${pkg} is not installed globally via npm. Trying non-npm update path...${NC}"
            if try_non_npm_update "$tool_name" "$pkg" "$bin_name"; then
                :
            else
                rc=$?
                if [[ $rc -eq 2 ]]; then
                    record_failure "${tool_name} (${pkg}, non-npm install detected but update method not found)"
                else
                    record_failure "${tool_name} (${pkg}, non-npm update failed)"
                fi
            fi
        else
            echo -e "${YELLOW}${pkg} is not installed globally via npm. Installing...${NC}"
            if run_npm_with_sudo_retry "install" "$pkg"; then
                record_success "${tool_name} (${pkg}, npm install -g)"
            elif [[ $? -eq 42 ]]; then
                echo -e "${YELLOW}Binary collision detected while installing ${pkg}. An existing executable already occupies the command path.${NC}"
                record_success "${tool_name} (${pkg}, install skipped due to existing binary)"
            else
                record_failure "${tool_name} (${pkg}, npm install -g)"
            fi
        fi
    done
}

agent_update() {
    echo -e "${BLUE}=== AI CLI Tools Update Start ===${NC}"
    
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

    # Copilot CLI extra check (only when npm-managed)
    if command -v copilot &> /dev/null && npm list -g --depth=0 "@github/copilot" &> /dev/null; then
        echo -e "${BLUE}Running copilot update check...${NC}"
        if copilot update; then
            record_success "copilot self-update"
        else
            echo -e "${YELLOW}Warning: copilot self-update failed.${NC}"
        fi
    fi

    summarize_results
    echo -e "${GREEN}=== AI CLI Tools Update Complete! ===${NC}"
}

show_help() {
    echo -e "${BLUE}one-click-tools (oct)${NC}"
    echo -e "Update and bootstrap popular AI CLI tools with a single command."
    echo -e ""
    echo -e "${YELLOW}Usage:${NC}"
    echo -e "  oct agent-update    Update all supported AI CLI agents"
    echo -e "  oct help            Show this help message"
    echo -e ""
    echo -e "${YELLOW}Supported Tools:${NC}"
    for tool in "${TOOLS[@]}"; do
        echo -e "  - ${tool}"
    done
}

# Command dispatcher
COMMAND="${1:-}"

case "$COMMAND" in
    agent-update)
        setup_logging_for_agent_update
        agent_update
        ;;
    help|--help|-h)
        show_help
        ;;
    "")
        show_help
        ;;
    *)
        echo -e "${RED}Unknown command: $COMMAND${NC}"
        show_help
        exit 1
        ;;
esac
