self_update() {
    local channel="${1:-stable}"
    local target_pkg="$SELF_PACKAGE_NAME"

    if ! command -v npm &> /dev/null; then
        warn_missing_manager_and_exit "npm"
    fi

    if [[ "$channel" == "beta" ]]; then
        target_pkg="${SELF_PACKAGE_NAME}@beta"
    fi

    echo -e "${BLUE}Updating one-click-tools via npm (${target_pkg})...${NC}"
    if run_npm_with_sudo_retry "install" "$target_pkg"; then
        echo -e "${GREEN}one-click-tools update complete (${target_pkg}).${NC}"
    else
        echo -e "${RED}one-click-tools update failed (${target_pkg}).${NC}"
        exit 1
    fi
}
