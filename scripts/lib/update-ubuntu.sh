update_ubuntu() {
    if ! command -v npm &> /dev/null; then
        warn_missing_manager_and_exit "npm"
    fi

    echo -e "${YELLOW}Detected Ubuntu. Updating npm global packages...${NC}"

    local -a enabled_indices=()
    local _i
    for _i in "${!TOOLS[@]}"; do
        is_tool_enabled "${BINARY_NAMES[$_i]}" && enabled_indices+=("$_i")
    done
    local _total=${#enabled_indices[@]}
    local _count=0

    for i in "${enabled_indices[@]}"; do
        tool_name="${TOOLS[$i]}"
        pkg="${NPM_PACKAGES[$i]}"
        bin_name="${BINARY_NAMES[$i]}"
        _count=$((_count + 1))
        echo -e "${BLUE}[${_count}/${_total}] Checking update for: ${tool_name} (${pkg})...${NC}"
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
