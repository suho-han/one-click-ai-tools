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
