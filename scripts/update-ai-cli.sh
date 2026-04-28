#!/bin/bash

# one-click-tools (oct)
# OS-aware updater for developer AI CLI tools

set -u

# ANSI Color Codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color
LOG_FILE=""

# Preserve original stdout/stderr so we can always print log location to terminal.
exec 3>&1
exec 4>&2

# Tool configuration
TOOLS=("Claude Code" "OpenAI Codex" "Gemini CLI" "GitHub Copilot")
NPM_PACKAGES=(
    "@anthropic-ai/claude-code"
    "@openai/codex"
    "@google/gemini-cli"
    "@github/copilot"
)
BINARY_NAMES=(
    "claude"
    "codex"
    "gemini"
    "copilot"
)
SELF_PACKAGE_NAME="one-click-tools"

SUCCESS_ITEMS=()
FAILED_ITEMS=()
USAGE_RESULTS=()
USAGE_EXPERIMENTAL_MODE=0

usage_json_escape() {
    local value="${1:-}"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    value="${value//$'\n'/\\n}"
    value="${value//$'\r'/\\r}"
    value="${value//$'\t'/\\t}"
    printf '%s' "$value"
}

append_usage_result() {
    local provider="$1"
    local period="$2"
    local used="$3"
    local limit="$4"
    local unit="$5"
    local source="$6"
    local status="$7"
    local message="$8"
    local source_detail="${9:-}"
    USAGE_RESULTS+=("${provider}|${period}|${used}|${limit}|${unit}|${source}|${status}|${message}|${source_detail}")
}

emit_usage_json() {
    local first=1
    printf '[\n'
    for row in "${USAGE_RESULTS[@]}"; do
        IFS='|' read -r provider period used limit unit source status message source_detail <<< "$row"
        if [[ $first -eq 0 ]]; then
            printf ',\n'
        fi
        first=0
        printf '  {"provider":"%s","period":"%s","used":"%s","limit":"%s","unit":"%s","source":"%s","status":"%s","message":"%s","source_detail":"%s"}' \
            "$(usage_json_escape "$provider")" \
            "$(usage_json_escape "$period")" \
            "$(usage_json_escape "$used")" \
            "$(usage_json_escape "$limit")" \
            "$(usage_json_escape "$unit")" \
            "$(usage_json_escape "$source")" \
            "$(usage_json_escape "$status")" \
            "$(usage_json_escape "$message")" \
            "$(usage_json_escape "$source_detail")"
    done
    printf '\n]\n'
}

emit_usage_table() {
    printf "%-12s %-12s %-12s %-12s %-10s %-8s %-8s %s\n" \
        "provider" "period" "used" "limit" "unit" "source" "status" "message"
    for row in "${USAGE_RESULTS[@]}"; do
        IFS='|' read -r provider period used limit unit source status message source_detail <<< "$row"
        printf "%-12s %-12s %-12s %-12s %-10s %-8s %-8s %s\n" \
            "$provider" "$period" "$used" "$limit" "$unit" "$source" "$status" "$message"
        if [[ -n "$source_detail" ]]; then
            printf "%-12s %-12s %-12s %-12s %-10s %-8s %-8s %s\n" \
                "" "" "" "" "" "" "" "source_detail=${source_detail}"
        fi
    done
}

extract_json_value_from_file() {
    local file_path="$1"
    local key="$2"
    if [[ ! -f "$file_path" ]]; then
        return 1
    fi
    if command -v jq &> /dev/null; then
        jq -r ".${key} // empty" "$file_path" 2>/dev/null | head -n1
    else
        sed -nE "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\"([^\"]+)\".*/\1/p" "$file_path" | head -n1
    fi
}

extract_first_number() {
    local text="$1"
    local number
    number="$(echo "$text" | grep -Eo '[0-9]+([.][0-9]+)?' | head -n1)"
    if [[ -n "$number" ]]; then
        printf '%s' "$number"
    fi
}

detect_unit_from_text() {
    local text="$1"
    local lowered
    lowered="$(printf '%s' "$text" | tr '[:upper:]' '[:lower:]')"
    if echo "$lowered" | grep -q "token"; then
        printf 'tokens'
        return
    fi
    if echo "$lowered" | grep -q "usd\\|\\$\\|cost"; then
        printf 'usd'
        return
    fi
    printf 'requests'
}

fetch_usage_from_api() {
    local provider="$1"
    local endpoint="$2"
    local auth_header="$3"

    if ! command -v curl &> /dev/null; then
        return 2
    fi

    local tmp_headers
    tmp_headers="$(mktemp)"
    echo "header = \"${auth_header}\"" > "$tmp_headers"

    local output
    output="$(curl -sS --config "$tmp_headers" "$endpoint" 2>&1)"
    local rc=$?
    rm -f "$tmp_headers"
    
    if [[ $rc -ne 0 ]]; then
        return 1
    fi

    local used limit period unit
    if command -v jq &> /dev/null; then
        used="$(printf '%s' "$output" | jq -r '(.used // .total_used // .usage // "n/a") | tostring' 2>/dev/null)"
        limit="$(printf '%s' "$output" | jq -r '(.limit // .quota // .max // "n/a") | tostring' 2>/dev/null)"
        period="$(printf '%s' "$output" | jq -r '(.period // .billing_period // .window // "current") | tostring' 2>/dev/null)"
        unit="$(printf '%s' "$output" | jq -r '(.unit // .metric // "") | tostring' 2>/dev/null)"
        [[ "$used" == "null" ]] && used="n/a"
        [[ "$limit" == "null" ]] && limit="n/a"
        [[ "$period" == "null" ]] && period="current"
    else
        used="$(printf '%s' "$output" | grep -Eo '"(used|total_used|usage)"[[:space:]]*:[[:space:]]*"?[0-9]+([.][0-9]+)?"?' | head -n1 | grep -Eo '[0-9]+([.][0-9]+)?')"
        limit="$(printf '%s' "$output" | grep -Eo '"(limit|quota|max)"[[:space:]]*:[[:space:]]*"?[0-9]+([.][0-9]+)?"?' | head -n1 | grep -Eo '[0-9]+([.][0-9]+)?')"
        period="$(printf '%s' "$output" | grep -Eo '"(period|billing_period|window)"[[:space:]]*:[[:space:]]*"[^"]+"' | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]+)"/\1/')"
        unit="$(printf '%s' "$output" | grep -Eo '"(unit|metric)"[[:space:]]*:[[:space:]]*"[^"]+"' | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]+)"/\1/')"
    fi

    [[ -z "$used" ]] && used="n/a"
    [[ -z "$limit" ]] && limit="n/a"
    [[ -z "$period" ]] && period="current"
    [[ -z "$unit" || "$unit" == "null" ]] && unit="$(detect_unit_from_text "$output")"

    append_usage_result "$provider" "$period" "$used" "$limit" "$unit" "api" "ok" "API usage fetched"
    return 0
}

fetch_usage_from_cli() {
    local provider="$1"
    local bin_name="$2"
    shift 2
    local -a commands=("$@")

    if ! command -v "$bin_name" &> /dev/null; then
        append_usage_result "$provider" "current" "n/a" "n/a" "requests" "cli" "error" "${bin_name} not installed"
        return 1
    fi

    local cmd output rc used limit unit
    for cmd in "${commands[@]}"; do
        local first_token second_token
        first_token="$(printf '%s' "$cmd" | awk '{print $1}')"
        second_token="$(printf '%s' "$cmd" | awk '{print $2}')"
        if [[ "$first_token" == "$bin_name" ]] && [[ -n "$second_token" ]]; then
            if ! supports_subcommand "$bin_name" "$second_token"; then
                continue
            fi
        fi

        output="$(run_command_capture_with_timeout "$cmd" 8)"
        rc=$?
        if [[ $rc -eq 0 ]] && [[ -n "$output" ]]; then
            used="$(extract_first_number "$output")"
            limit="$(echo "$output" | grep -Ei 'limit|quota|max' | head -n1 | grep -Eo '[0-9]+([.][0-9]+)?')"
            unit="$(detect_unit_from_text "$output")"
            [[ -z "$used" ]] && used="n/a"
            [[ -z "$limit" ]] && limit="n/a"
            append_usage_result "$provider" "current" "$used" "$limit" "$unit" "cli" "ok" "CLI usage fetched"
            return 0
        fi
    done

    append_usage_result "$provider" "current" "n/a" "n/a" "requests" "cli" "error" "No usage command exposed by CLI; set OCT_*_USAGE_ENDPOINT"
    return 1
}

run_command_capture_with_timeout() {
    local cmd="$1"
    local timeout_secs="${2:-8}"
    local tmp_file
    tmp_file="$(mktemp)"

    (
        # shellcheck disable=SC2086
        eval "$cmd"
    ) >"$tmp_file" 2>&1 &
    local cmd_pid=$!

    local elapsed=0
    while kill -0 "$cmd_pid" 2>/dev/null; do
        if [[ $elapsed -ge $timeout_secs ]]; then
            if command -v pkill &> /dev/null; then
                pkill -P "$cmd_pid" 2>/dev/null || true
            fi
            kill "$cmd_pid" 2>/dev/null || true
            wait "$cmd_pid" 2>/dev/null || true
            cat "$tmp_file"
            rm -f "$tmp_file"
            return 124
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done

    wait "$cmd_pid"
    local rc=$?
    cat "$tmp_file"
    rm -f "$tmp_file"
    return $rc
}

supports_subcommand() {
    local bin_name="$1"
    local subcommand="$2"
    local help_text

    if ! command -v "$bin_name" &> /dev/null; then
        return 1
    fi

    help_text="$("$bin_name" --help 2>&1)"
    echo "$help_text" | grep -Eq "(^|[[:space:]])${subcommand}([[:space:]]|$)"
}

fetch_usage_claude_experimental() {
    local token=""
    local credentials_file="${HOME}/.claude/.credentials.json"
    local endpoint="https://api.anthropic.com/api/oauth/usage"

    token="$(extract_json_value_from_file "$credentials_file" "access_token")"
    if [[ -z "$token" ]]; then
        token="$(extract_json_value_from_file "$credentials_file" "accessToken")"
    fi

    if [[ -z "$token" ]] && command -v security &> /dev/null; then
        token="$(security find-generic-password -w -s "Claude Code-credentials" 2>/dev/null)"
    fi

    if [[ -z "$token" ]]; then
        append_usage_result "claude-code" "current" "n/a" "n/a" "requests" "oauth" "error" "No Claude OAuth token from credentials/keychain" "experimental_oauth_api"
        return 1
    fi

    local output
    output="$(curl -sS \
        -H "Authorization: Bearer ${token}" \
        -H "Accept: application/json" \
        -H "Content-Type: application/json" \
        -H "anthropic-beta: oauth-2025-04-20" \
        -H "User-Agent: one-click-tools/experimental" \
        "$endpoint" 2>&1)"
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        append_usage_result "claude-code" "current" "n/a" "n/a" "requests" "oauth" "error" "Claude experimental API request failed" "experimental_oauth_api"
        return 1
    fi

    local five_hour seven_day
    five_hour="$(printf '%s' "$output" | grep -Eo '"five_hour"[^{]*\{[^}]*"utilization"[[:space:]]*:[[:space:]]*[0-9]+([.][0-9]+)?' | grep -Eo '[0-9]+([.][0-9]+)?' | tail -n1)"
    seven_day="$(printf '%s' "$output" | grep -Eo '"seven_day"[^{]*\{[^}]*"utilization"[[:space:]]*:[[:space:]]*[0-9]+([.][0-9]+)?' | grep -Eo '[0-9]+([.][0-9]+)?' | tail -n1)"

    if [[ -z "$five_hour" ]] && [[ -z "$seven_day" ]]; then
        append_usage_result "claude-code" "current" "n/a" "n/a" "requests" "oauth" "error" "Claude experimental API parse failed" "experimental_oauth_api"
        return 1
    fi

    local used="n/a"
    local msg_parts=()
    if [[ -n "$five_hour" ]]; then
        used="${five_hour}"
        msg_parts+=("five_hour=${five_hour}%")
    fi
    if [[ -n "$seven_day" ]]; then
        msg_parts+=("seven_day=${seven_day}%")
    fi

    append_usage_result "claude-code" "current" "$used" "100" "percent" "oauth" "ok" "${msg_parts[*]}" "experimental_oauth_api"
    return 0
}

fetch_usage_codex_experimental() {
    local auth_file="${HOME}/.codex/auth.json"
    local session_dir="${HOME}/.codex/sessions"
    local used
    local plan_type

    if [[ ! -f "$auth_file" ]] && [[ ! -d "$session_dir" ]]; then
        append_usage_result "codex" "current" "n/a" "n/a" "percent" "local" "error" "No Codex auth/session data found" "experimental_local_aggregate"
        return 1
    fi

    used="$(find "$session_dir" -type f -name "*.jsonl" 2>/dev/null | xargs -I{} tail -n 200 "{}" 2>/dev/null | grep -Eo '"used_percent"[[:space:]]*:[[:space:]]*[0-9]+([.][0-9]+)?' | tail -n1 | grep -Eo '[0-9]+([.][0-9]+)?')"
    plan_type="$(extract_json_value_from_file "$auth_file" "plan_type")"

    if [[ -z "$used" ]]; then
        append_usage_result "codex" "current" "n/a" "n/a" "percent" "local" "error" "Codex local aggregate unavailable from session logs" "experimental_local_aggregate"
        return 1
    fi

    if [[ -n "$plan_type" ]]; then
        append_usage_result "codex" "current" "$used" "100" "percent" "local" "ok" "Used percent from local session logs" "experimental_local_aggregate plan=${plan_type}"
    else
        append_usage_result "codex" "current" "$used" "100" "percent" "local" "ok" "Used percent from local session logs" "experimental_local_aggregate"
    fi
    return 0
}

fetch_usage_gemini_experimental() {
    local oauth_file="${HOME}/.gemini/oauth_creds.json"
    local endpoint="${OCT_GEMINI_CODEASSIST_ENDPOINT:-}"
    local token

    token="$(extract_json_value_from_file "$oauth_file" "access_token")"
    if [[ -z "$token" ]]; then
        token="$(extract_json_value_from_file "$oauth_file" "accessToken")"
    fi

    if [[ -z "$token" ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "No Gemini OAuth token in ~/.gemini/oauth_creds.json" "experimental_oauth_api"
        return 1
    fi

    if [[ -z "$endpoint" ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "Set OCT_GEMINI_CODEASSIST_ENDPOINT for experimental Gemini quota API" "experimental_oauth_api"
        return 1
    fi

    local output
    output="$(curl -sS -H "Authorization: Bearer ${token}" -H "Accept: application/json" "$endpoint" 2>&1)"
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "Gemini experimental API request failed" "experimental_oauth_api"
        return 1
    fi

    local used limit unit
    used="$(printf '%s' "$output" | grep -Eo '"(used|usage|currentUsage)"[[:space:]]*:[[:space:]]*[0-9]+([.][0-9]+)?' | head -n1 | grep -Eo '[0-9]+([.][0-9]+)?')"
    limit="$(printf '%s' "$output" | grep -Eo '"(limit|quota|max|total)"[[:space:]]*:[[:space:]]*[0-9]+([.][0-9]+)?' | head -n1 | grep -Eo '[0-9]+([.][0-9]+)?')"
    unit="$(detect_unit_from_text "$output")"

    if [[ -z "$used" ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "Gemini experimental API parse failed" "experimental_oauth_api"
        return 1
    fi
    [[ -z "$limit" ]] && limit="n/a"
    append_usage_result "gemini" "current" "$used" "$limit" "$unit" "oauth" "ok" "Gemini experimental API usage fetched" "experimental_oauth_api"
    return 0
}

get_usage_codex() {
    if [[ "${USAGE_EXPERIMENTAL_MODE}" -eq 1 ]]; then
        if fetch_usage_codex_experimental; then
            return 0
        fi
    fi
    local api_key="${OPENAI_API_KEY:-}"
    local endpoint="${OCT_CODEX_USAGE_ENDPOINT:-}"
    if [[ -n "$api_key" ]] && [[ -n "$endpoint" ]]; then
        if fetch_usage_from_api "codex" "$endpoint" "Authorization: Bearer ${api_key}"; then
            return 0
        fi
    fi
    fetch_usage_from_cli "codex" "codex" \
        "codex usage --json" \
        "codex usage" \
        "codex billing"
}

get_usage_claude() {
    if [[ "${USAGE_EXPERIMENTAL_MODE}" -eq 1 ]]; then
        if fetch_usage_claude_experimental; then
            return 0
        fi
    fi
    local api_key="${ANTHROPIC_API_KEY:-}"
    local endpoint="${OCT_CLAUDE_USAGE_ENDPOINT:-}"
    if [[ -n "$api_key" ]] && [[ -n "$endpoint" ]]; then
        if fetch_usage_from_api "claude-code" "$endpoint" "x-api-key: ${api_key}"; then
            return 0
        fi
    fi
    fetch_usage_from_cli "claude-code" "claude" \
        "claude usage --json" \
        "claude usage" \
        "claude billing"
}

get_usage_gemini() {
    if [[ "${USAGE_EXPERIMENTAL_MODE}" -eq 1 ]]; then
        if fetch_usage_gemini_experimental; then
            return 0
        fi
    fi
    local api_key="${GEMINI_API_KEY:-${GOOGLE_API_KEY:-}}"
    local endpoint="${OCT_GEMINI_USAGE_ENDPOINT:-}"
    if [[ -n "$api_key" ]] && [[ -n "$endpoint" ]]; then
        if fetch_usage_from_api "gemini" "$endpoint" "x-goog-api-key: ${api_key}"; then
            return 0
        fi
    fi
    fetch_usage_from_cli "gemini" "gemini" \
        "gemini usage --json" \
        "gemini usage" \
        "gemini billing"
}

get_usage_all() {
    local output_mode="${1:-table}"
    local ok_count=0
    USAGE_RESULTS=()

    get_usage_codex && ok_count=$((ok_count + 1))
    get_usage_claude && ok_count=$((ok_count + 1))
    get_usage_gemini && ok_count=$((ok_count + 1))

    if [[ "$output_mode" == "json" ]]; then
        emit_usage_json
    else
        emit_usage_table
    fi

    if [[ $ok_count -eq 0 ]]; then
        return 1
    fi
    return 0
}

usage_command() {
    local output_mode="table"
    USAGE_EXPERIMENTAL_MODE=0

    while [[ $# -gt 0 ]]; do
        if [[ -z "${1:-}" ]]; then
            shift
            continue
        fi
        case "$1" in
            --json)
                output_mode="json"
                ;;
            --experimental-oauth-usage)
                USAGE_EXPERIMENTAL_MODE=1
                ;;
            *)
                echo -e "${RED}Unknown option for usage: ${1}${NC}"
                return 1
                ;;
        esac
        shift
    done

    if [[ "${USAGE_EXPERIMENTAL_MODE}" -eq 1 ]]; then
        echo -e "${YELLOW}Warning: experimental OAuth/local usage mode enabled. Provider APIs/formats may change without notice.${NC}"
    fi

    get_usage_all "$output_mode"
}

record_success() {
    SUCCESS_ITEMS+=("$1")
}

record_failure() {
    FAILED_ITEMS+=("$1")
    echo -e "${RED}Failed: $1${NC}"
}

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

summarize_results() {
    echo -e "${BLUE}=== Summary ===${NC}"
    echo -e "${GREEN}Succeeded: ${#SUCCESS_ITEMS[@]}${NC}"
    for item in "${SUCCESS_ITEMS[@]}"; do
        echo -e "  ${GREEN}- ${item}${NC}"
    done

    if [[ ${#FAILED_ITEMS[@]} -gt 0 ]]; then
        echo -e "${RED}Failed: ${#FAILED_ITEMS[@]}${NC}"
        for item in "${FAILED_ITEMS[@]}"; do
            echo -e "  ${RED}- ${item}${NC}"
        done
    else
        echo -e "${GREEN}Failed: 0${NC}"
    fi
}

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
        sudo "$(command -v npm)" "$action" -g "$pkg"
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
        sudo "$(command -v npm)" install -g --force "$pkg"
        return $?
    fi

    return $rc
}

binary_exists() {
    local bin_name="$1"
    command -v "$bin_name" &> /dev/null
}

try_brew_update_for_tool() {
    local tool_name="$1"
    local pkg="$2"
    local candidates=()
    local candidate

    if ! command -v brew &> /dev/null; then
        return 2
    fi

    case "$pkg" in
        "@anthropic-ai/claude-code")
            candidates=("claude-code")
            ;;
        "@openai/codex")
            candidates=("codex")
            ;;
        "@google/gemini-cli")
            candidates=("gemini-cli" "gemini")
            ;;
        "@github/copilot")
            candidates=("copilot")
            ;;
        *)
            return 2
            ;;
    esac

    for candidate in "${candidates[@]}"; do
        if brew list --formula "$candidate" &> /dev/null; then
            echo -e "${YELLOW}${tool_name} appears to be Homebrew formula-managed (${candidate}). Running brew upgrade ${candidate}...${NC}"
            if brew upgrade "$candidate"; then
                record_success "${tool_name} (${pkg}, brew formula upgrade: ${candidate})"
                return 0
            fi
            return 1
        fi

        if brew list --cask "$candidate" &> /dev/null; then
            echo -e "${YELLOW}${tool_name} appears to be Homebrew cask-managed (${candidate}). Running brew upgrade --cask ${candidate}...${NC}"
            if brew upgrade --cask "$candidate"; then
                record_success "${tool_name} (${pkg}, brew cask upgrade: ${candidate})"
                return 0
            fi
            return 1
        fi
    done

    return 2
}

try_non_npm_update() {
    local tool_name="$1"
    local pkg="$2"
    local bin_name="$3"

    if [[ "$pkg" == "@github/copilot" ]]; then
        echo -e "${YELLOW}${tool_name} non-npm install detected. Forcing npm install to align with npm version...${NC}"
        if run_npm_force_install_with_sudo_retry "$pkg"; then
            record_success "${tool_name} (${pkg}, npm install -g --force to align version)"
            return 0
        fi
        echo -e "${YELLOW}Warning: npm force install for ${tool_name} failed. Falling back to non-npm update path.${NC}"
    fi

    if try_brew_update_for_tool "$tool_name" "$pkg"; then
        return 0
    fi

    local brew_rc=$?
    if [[ $brew_rc -eq 1 ]]; then
        return 1
    fi

    if [[ "$pkg" == "@github/copilot" ]] && command -v "$bin_name" &> /dev/null; then
        echo -e "${YELLOW}${tool_name} appears to be self-update capable. Running ${bin_name} update...${NC}"
        if "$bin_name" update; then
            record_success "${tool_name} (${pkg}, self-update)"
            return 0
        fi
        return 1
    fi

    return 2
}

update_macos() {
    if ! command -v brew &> /dev/null; then
        warn_missing_manager_and_exit "brew"
    fi

    if ! command -v npm &> /dev/null; then
        warn_missing_manager_and_exit "npm"
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

    echo -e "${BLUE}Caching global npm packages...${NC}"
    local global_npm_packages
    global_npm_packages="$(npm list -g --depth=0 2>/dev/null || true)"

    for i in "${!TOOLS[@]}"; do
        tool_name="${TOOLS[$i]}"
        pkg="${NPM_PACKAGES[$i]}"
        bin_name="${BINARY_NAMES[$i]}"
        echo -e "${BLUE}Checking update for: ${tool_name} (${pkg})...${NC}"
        
        if echo "$global_npm_packages" | grep -qE "(^|[[:space:]])${pkg}@" || echo "$global_npm_packages" | grep -qE "(^|[[:space:]])${pkg}$"; then
            echo -e "${YELLOW}Upgrading ${pkg}...${NC}"
            if run_npm_with_sudo_retry "update" "$pkg"; then
                record_success "${tool_name} (${pkg}, npm update -g)"
            else
                echo -e "${YELLOW}npm update failed for ${pkg}. Trying Homebrew fallback on macOS...${NC}"
                local _brew_rc=0
                try_brew_update_for_tool "$tool_name" "$pkg" || _brew_rc=$?
                if [[ $_brew_rc -eq 0 ]]; then
                    : # record_success already called inside try_brew_update_for_tool
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
                : # record_success already called inside try_brew_update_for_tool
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
                        : # record_success already called inside try_brew_update_for_tool
                    else
                        record_failure "${tool_name} (${pkg}, npm install -g; brew fallback failed)"
                    fi
                fi
            fi
        fi
    done
}

update_ubuntu() {
    if ! command -v npm &> /dev/null; then
        warn_missing_manager_and_exit "npm"
    fi

    echo -e "${YELLOW}Detected Ubuntu. Updating npm global packages...${NC}"

    echo -e "${BLUE}Caching global npm packages...${NC}"
    local global_npm_packages
    global_npm_packages="$(npm list -g --depth=0 2>/dev/null || true)"

    for i in "${!TOOLS[@]}"; do
        tool_name="${TOOLS[$i]}"
        pkg="${NPM_PACKAGES[$i]}"
        bin_name="${BINARY_NAMES[$i]}"

        echo -e "${BLUE}Checking update for: ${tool_name} (${pkg})...${NC}"
        if echo "$global_npm_packages" | grep -qE "(^|[[:space:]])${pkg}@" || echo "$global_npm_packages" | grep -qE "(^|[[:space:]])${pkg}$"; then
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

show_help() {
    echo -e "${BLUE}one-click-tools (oct)${NC}"
    echo -e "Update and bootstrap popular AI CLI tools with a single command."
    echo -e ""
    echo -e "${YELLOW}Usage:${NC}"
    echo -e "  oct update          Update oct itself to latest stable"
    echo -e "  oct update --beta   Update oct itself to latest beta"
    echo -e "  oct agent-update    Update all supported AI CLI agents"
    echo -e "  oct usage [--json] [--experimental-oauth-usage]  Show codex/claude/gemini usage summary"
    echo -e "  oct help            Show this help message"
    echo -e ""
    echo -e "${YELLOW}Supported Tools:${NC}"
    for tool in "${TOOLS[@]}"; do
        echo -e "  - ${tool}"
    done
}

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
