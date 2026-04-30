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
                echo -e "${YELLOW}npm update failed for ${pkg}. Trying Homebrew fallback on macOS...${NC}"
                local _brew_rc=0
                try_brew_update_for_tool "$tool_name" "$pkg" || _brew_rc=$?
                if [[ $_brew_rc -eq 0 ]]; then
                    :
                else
                    record_failure "${tool_name} (${pkg}, npm update -g; brew fallback failed)"
                fi
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
            # Try Homebrew first — brew list --cask works even if the symlink is broken or missing from PATH
            local _brew_rc=0
            try_brew_update_for_tool "$tool_name" "$pkg" || _brew_rc=$?
            if [[ $_brew_rc -eq 0 ]]; then
                :
            elif [[ $_brew_rc -eq 1 ]]; then
                record_failure "${tool_name} (${pkg}, brew upgrade)"
            else
                # Not a Homebrew package — fall back to npm install
                echo -e "${YELLOW}${pkg} is not installed globally via npm. Installing...${NC}"
                local _npm_rc=0
                run_npm_with_sudo_retry "install" "$pkg" || _npm_rc=$?
                if [[ $_npm_rc -eq 0 ]]; then
                    record_success "${tool_name} (${pkg}, npm install -g)"
                elif [[ $_npm_rc -eq 42 ]]; then
                    echo -e "${YELLOW}Binary collision detected while installing ${pkg}. An existing executable already occupies the command path.${NC}"
                    record_success "${tool_name} (${pkg}, install skipped due to existing binary)"
                else
                    echo -e "${YELLOW}npm install failed for ${pkg}. Trying Homebrew fallback on macOS...${NC}"
                    local _fallback_brew_rc=0
                    try_brew_update_for_tool "$tool_name" "$pkg" || _fallback_brew_rc=$?
                    if [[ $_fallback_brew_rc -eq 0 ]]; then
                        :
                    else
                        record_failure "${tool_name} (${pkg}, npm install -g; brew fallback failed)"
                    fi
                fi
            fi
        fi
    done
}
