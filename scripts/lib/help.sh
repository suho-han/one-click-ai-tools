show_help() {
    echo -e "${BLUE}one-click-tools (oct)${NC}"
    echo -e "Update and bootstrap popular AI CLI tools with a single command."
    echo -e ""
    echo -e "${YELLOW}Usage:${NC}"
    echo -e "  oct update                Update oct itself to latest stable"
    echo -e "  oct update --beta         Update oct itself to latest beta"
    echo -e "  oct agent-update          Update all supported AI CLI agents"
    echo -e "  oct usage [--json] [--experimental-oauth-usage]"
    echo -e "                            Show codex/claude/gemini/copilot usage summary"
    echo -e "  oct config                Show current config"
    echo -e "  oct config set tools <list>  Set tools to update (e.g. claude,codex)"
    echo -e "  oct config reset          Reset config to defaults"
    echo -e "  oct schedule              Show schedule status"
    echo -e "  oct schedule enable [--daily|--weekly] [--hour 9]"
    echo -e "                            Enable scheduled auto-update"
    echo -e "  oct schedule disable      Disable scheduled auto-update"
    echo -e "  oct help                  Show this help message"
    echo -e ""
    echo -e "${YELLOW}Supported Tools:${NC}"
    for tool in "${TOOLS[@]}"; do
        echo -e "  - ${tool}"
    done
}
