OCT_PLIST_LABEL="com.oct.agent-update"
OCT_PLIST_PATH="$HOME/Library/LaunchAgents/${OCT_PLIST_LABEL}.plist"

_get_oct_bin_path() {
    command -v oct 2>/dev/null || echo "oct"
}

_generate_plist() {
    local interval="$1"
    local hour="$2"
    local oct_bin
    oct_bin="$(_get_oct_bin_path)"

    local interval_block
    if [[ "$interval" == "weekly" ]]; then
        interval_block="<dict>
            <key>Hour</key><integer>${hour}</integer>
            <key>Minute</key><integer>0</integer>
            <key>Weekday</key><integer>1</integer>
        </dict>"
    else
        interval_block="<dict>
            <key>Hour</key><integer>${hour}</integer>
            <key>Minute</key><integer>0</integer>
        </dict>"
    fi

    cat << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${OCT_PLIST_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${oct_bin}</string>
        <string>agent-update</string>
    </array>
    <key>StartCalendarInterval</key>
    ${interval_block}
    <key>StandardOutPath</key>
    <string>${HOME}/.oct/logs/schedule.log</string>
    <key>StandardErrorPath</key>
    <string>${HOME}/.oct/logs/schedule.log</string>
</dict>
</plist>
EOF
}

_save_schedule_config() {
    local interval="$1"
    local hour="$2"
    mkdir -p "$OCT_CONFIG_DIR"

    local tmp_file
    tmp_file="$(mktemp)"
    grep -v -E '^schedule_' "$OCT_CONFIG_FILE" 2>/dev/null > "$tmp_file" || true
    {
        echo "schedule_enabled=true"
        echo "schedule_interval=${interval}"
        echo "schedule_hour=${hour}"
    } >> "$tmp_file"
    mv "$tmp_file" "$OCT_CONFIG_FILE"
}

_disable_schedule_config() {
    if [[ ! -f "$OCT_CONFIG_FILE" ]]; then
        return
    fi
    local tmp_file
    tmp_file="$(mktemp)"
    grep -v -E '^schedule_' "$OCT_CONFIG_FILE" > "$tmp_file" || true
    echo "schedule_enabled=false" >> "$tmp_file"
    mv "$tmp_file" "$OCT_CONFIG_FILE"
}

_enable_schedule_macos() {
    local interval="$1"
    local hour="$2"

    mkdir -p "$HOME/.oct/logs"
    mkdir -p "$(dirname "$OCT_PLIST_PATH")"

    if [[ -f "$OCT_PLIST_PATH" ]]; then
        launchctl unload "$OCT_PLIST_PATH" 2>/dev/null || true
    fi

    _generate_plist "$interval" "$hour" > "$OCT_PLIST_PATH"
    launchctl load "$OCT_PLIST_PATH"

    echo -e "${GREEN}Schedule enabled (${interval}, ${hour}:00).${NC}"
    echo -e "Logs: ${HOME}/.oct/logs/schedule.log"
}

_enable_schedule_linux() {
    local interval="$1"
    local hour="$2"
    local oct_bin
    oct_bin="$(_get_oct_bin_path)"

    mkdir -p "$HOME/.oct/logs"

    local cron_expr
    if [[ "$interval" == "weekly" ]]; then
        cron_expr="0 ${hour} * * 1"
    else
        cron_expr="0 ${hour} * * *"
    fi

    local cron_entry="${cron_expr} ${oct_bin} agent-update >> ${HOME}/.oct/logs/schedule.log 2>&1"
    (crontab -l 2>/dev/null | grep -v "oct agent-update"; echo "$cron_entry") | crontab -

    echo -e "${GREEN}Schedule enabled (${interval}, ${hour}:00).${NC}"
    echo -e "Logs: ${HOME}/.oct/logs/schedule.log"
}

enable_schedule() {
    local interval="daily"
    local hour="9"

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --daily)  interval="daily"; shift ;;
            --weekly) interval="weekly"; shift ;;
            --hour)
                shift
                hour="${1:-9}"
                shift
                ;;
            *)
                echo -e "${RED}Unknown option: $1${NC}"
                echo -e "Usage: oct schedule enable [--daily|--weekly] [--hour <0-23>]"
                exit 1
                ;;
        esac
    done

    local os_name
    os_name="$(uname -s)"
    case "$os_name" in
        Darwin) _enable_schedule_macos "$interval" "$hour" ;;
        Linux)  _enable_schedule_linux "$interval" "$hour" ;;
        *)
            echo -e "${RED}Schedule is only supported on macOS and Linux.${NC}"
            exit 1
            ;;
    esac

    _save_schedule_config "$interval" "$hour"
}

disable_schedule() {
    local os_name
    os_name="$(uname -s)"

    case "$os_name" in
        Darwin)
            if [[ -f "$OCT_PLIST_PATH" ]]; then
                launchctl unload "$OCT_PLIST_PATH" 2>/dev/null || true
                rm -f "$OCT_PLIST_PATH"
                echo -e "${GREEN}Schedule disabled.${NC}"
            else
                echo -e "${YELLOW}No active schedule found.${NC}"
            fi
            ;;
        Linux)
            if crontab -l 2>/dev/null | grep -q "oct agent-update"; then
                (crontab -l 2>/dev/null | grep -v "oct agent-update") | crontab -
                echo -e "${GREEN}Schedule disabled.${NC}"
            else
                echo -e "${YELLOW}No active schedule found.${NC}"
            fi
            ;;
        *)
            echo -e "${RED}Schedule is only supported on macOS and Linux.${NC}"
            exit 1
            ;;
    esac

    _disable_schedule_config
}

show_schedule() {
    local os_name
    os_name="$(uname -s)"

    echo -e "${BLUE}=== Schedule Status ===${NC}"

    local is_enabled=false
    case "$os_name" in
        Darwin)
            if [[ -f "$OCT_PLIST_PATH" ]] && launchctl list 2>/dev/null | grep -q "$OCT_PLIST_LABEL"; then
                is_enabled=true
            fi
            ;;
        Linux)
            if crontab -l 2>/dev/null | grep -q "oct agent-update"; then
                is_enabled=true
            fi
            ;;
    esac

    if [[ "$is_enabled" == "true" ]]; then
        local interval hour
        interval="$(grep -E '^schedule_interval=' "$OCT_CONFIG_FILE" 2>/dev/null | cut -d= -f2 || echo "daily")"
        hour="$(grep -E '^schedule_hour=' "$OCT_CONFIG_FILE" 2>/dev/null | cut -d= -f2 || echo "9")"
        echo -e "  Status:   ${GREEN}enabled${NC}"
        echo -e "  Interval: ${interval}"
        echo -e "  Time:     ${hour}:00"
        echo -e "  Logs:     ${HOME}/.oct/logs/schedule.log"
    else
        echo -e "  Status:   ${YELLOW}disabled${NC}"
        echo ""
        echo -e "  To enable: oct schedule enable [--daily|--weekly] [--hour 9]"
    fi
}

schedule_command() {
    local subcmd="${1:-}"
    shift || true
    case "$subcmd" in
        "")
            show_schedule
            ;;
        enable)
            enable_schedule "$@"
            ;;
        disable)
            disable_schedule
            ;;
        *)
            echo -e "${RED}Unknown schedule subcommand: ${subcmd}${NC}"
            echo -e "Usage: oct schedule [enable [--daily|--weekly] [--hour 9]|disable]"
            exit 1
            ;;
    esac
}
