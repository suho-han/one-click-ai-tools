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
