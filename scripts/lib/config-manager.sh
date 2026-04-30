OCT_CONFIG_DIR="$HOME/.oct"
OCT_CONFIG_FILE="$OCT_CONFIG_DIR/config"
ENABLED_TOOLS=()

_oct_load_config() {
    if [[ ! -f "$OCT_CONFIG_FILE" ]]; then
        ENABLED_TOOLS=()
        return
    fi

    local tools_line
    tools_line="$(grep -E '^enabled_tools=' "$OCT_CONFIG_FILE" 2>/dev/null || true)"
    if [[ -n "$tools_line" ]]; then
        local tools_value="${tools_line#enabled_tools=}"
        if [[ "$tools_value" == "all" ]] || [[ -z "$tools_value" ]]; then
            ENABLED_TOOLS=()
        else
            IFS=',' read -ra ENABLED_TOOLS <<< "$tools_value"
        fi
    fi
}

is_tool_enabled() {
    local bin_name="$1"
    if [[ ${#ENABLED_TOOLS[@]} -eq 0 ]]; then
        return 0
    fi
    local enabled
    for enabled in "${ENABLED_TOOLS[@]}"; do
        [[ "$enabled" == "$bin_name" ]] && return 0
    done
    return 1
}

_oct_save_config() {
    mkdir -p "$OCT_CONFIG_DIR"
    local tools_value
    if [[ ${#ENABLED_TOOLS[@]} -eq 0 ]]; then
        tools_value="all"
    else
        tools_value="$(IFS=','; echo "${ENABLED_TOOLS[*]}")"
    fi

    local tmp_file
    tmp_file="$(mktemp)"
    grep -v -E '^enabled_tools=' "$OCT_CONFIG_FILE" 2>/dev/null > "$tmp_file" || true
    echo "enabled_tools=${tools_value}" >> "$tmp_file"
    mv "$tmp_file" "$OCT_CONFIG_FILE"
}

show_config() {
    _oct_load_config

    echo -e "${BLUE}=== one-click-tools config ===${NC}"
    echo -e "${YELLOW}Config file:${NC} ${OCT_CONFIG_FILE}"
    echo ""
    echo -e "${YELLOW}Enabled tools (agent-update):${NC}"

    local i tool_name bin_name
    for i in "${!TOOLS[@]}"; do
        tool_name="${TOOLS[$i]}"
        bin_name="${BINARY_NAMES[$i]}"
        if is_tool_enabled "$bin_name"; then
            echo -e "  ${GREEN}✓ ${tool_name}${NC}"
        else
            echo -e "  ${RED}✗ ${tool_name}${NC}"
        fi
    done
}

_set_config_tools() {
    local input="$1"
    local -a requested
    IFS=',' read -ra requested <<< "$input"

    local -a valid_tools=()
    local req bin_name found
    for req in "${requested[@]}"; do
        req="${req// /}"
        found=0
        for bin_name in "${BINARY_NAMES[@]}"; do
            if [[ "$req" == "$bin_name" ]]; then
                found=1
                valid_tools+=("$req")
                break
            fi
        done
        if [[ $found -eq 0 ]]; then
            echo -e "${RED}Unknown tool: ${req}${NC}"
            echo -e "Valid options: $(IFS=', '; echo "${BINARY_NAMES[*]}")"
            return 1
        fi
    done

    ENABLED_TOOLS=("${valid_tools[@]}")
    _oct_save_config
    echo -e "${GREEN}Config updated.${NC}"
    show_config
}

_reset_config() {
    ENABLED_TOOLS=()
    _oct_save_config
    echo -e "${GREEN}Config reset to defaults (all tools enabled).${NC}"
    show_config
}

config_command() {
    local subcmd="${1:-}"
    case "$subcmd" in
        "")
            show_config
            ;;
        set)
            local key="${2:-}"
            local value="${3:-}"
            case "$key" in
                tools)
                    if [[ -z "$value" ]]; then
                        echo -e "${RED}Usage: oct config set tools <tool1,tool2,...>${NC}"
                        echo -e "Available: $(IFS=', '; echo "${BINARY_NAMES[*]}")"
                        exit 1
                    fi
                    _set_config_tools "$value"
                    ;;
                *)
                    echo -e "${RED}Unknown config key: ${key}${NC}"
                    echo -e "Available keys: tools"
                    exit 1
                    ;;
            esac
            ;;
        reset)
            _reset_config
            ;;
        *)
            echo -e "${RED}Unknown config subcommand: ${subcmd}${NC}"
            echo -e "Usage: oct config [set tools <list>|reset]"
            exit 1
            ;;
    esac
}

_oct_load_config
