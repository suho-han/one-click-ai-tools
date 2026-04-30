#!/bin/bash

# one-click-tools (oct)
# OS-aware updater for developer AI CLI tools

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="${SCRIPT_DIR}/lib"

LOG_FILE=""

# Preserve original stdout/stderr so we can always print log location to terminal.
exec 3>&1
exec 4>&2

source "${LIB_DIR}/colors.sh"
source "${LIB_DIR}/config.sh"
source "${LIB_DIR}/results.sh"
source "${LIB_DIR}/logging.sh"
source "${LIB_DIR}/npm.sh"
source "${LIB_DIR}/package-managers.sh"
source "${LIB_DIR}/update-macos.sh"
source "${LIB_DIR}/update-ubuntu.sh"
source "${LIB_DIR}/agent-update.sh"
source "${LIB_DIR}/self-update.sh"
source "${LIB_DIR}/usage-report.sh"
source "${LIB_DIR}/help.sh"

# Command dispatcher
COMMAND="${1:-}"
SUBCOMMAND_OPT="${2:-}"

case "$COMMAND" in
    update)
        case "$SUBCOMMAND_OPT" in
            "")
                self_update "stable"
                ;;
            --beta)
                self_update "beta"
                ;;
            *)
                echo -e "${RED}Unknown option for update: ${SUBCOMMAND_OPT}${NC}"
                show_help
                exit 1
                ;;
        esac
        ;;
    agent-update)
        setup_logging_for_agent_update
        agent_update
        ;;
    usage)
        usage_command "${@:2}"
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
