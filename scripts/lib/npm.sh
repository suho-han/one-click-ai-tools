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
