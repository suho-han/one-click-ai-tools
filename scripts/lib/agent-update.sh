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
