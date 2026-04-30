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

sanitize_token() {
    local token="${1:-}"
    token="${token#Bearer }"
    token="${token#bearer }"
    token="$(printf '%s' "$token" | tr -d '\r' | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//')"
    printf '%s' "$token"
}

add_token_candidate() {
    local candidate
    candidate="$(sanitize_token "${1:-}")"
    [[ -z "$candidate" ]] && return 0
    local existing
    for existing in "${token_candidates[@]:-}"; do
        [[ "$existing" == "$candidate" ]] && return 0
    done
    token_candidates+=("$candidate")
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
    sed -nE "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\"([^\"]+)\".*/\1/p" "$file_path" | head -n1
}

extract_env_value_from_dotenv() {
    local key="$1"
    local dotenv_path="${2:-.env}"
    local -a dotenv_candidates=()

    if [[ -n "${2:-}" ]]; then
        dotenv_candidates+=("$dotenv_path")
    else
        dotenv_candidates+=(".env" "${HOME}/.env")
    fi

    local candidate value
    for candidate in "${dotenv_candidates[@]}"; do
        [[ -f "$candidate" ]] || continue
        value="$(sed -nE "s/^[[:space:]]*${key}[[:space:]]*=[[:space:]]*\"?([^\"]*)\"?[[:space:]]*$/\\1/p" "$candidate" | head -n1)"
        if [[ -n "$value" ]]; then
            printf '%s' "$value"
            return 0
        fi
    done
    return 1
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

    local output
    output="$(curl -sS -H "$auth_header" "$endpoint" 2>&1)"
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        return 1
    fi

    local used limit period unit
    used="$(printf '%s' "$output" | grep -Eo '"(used|total_used|usage)"[[:space:]]*:[[:space:]]*"?[0-9]+([.][0-9]+)?"?' | head -n1 | grep -Eo '[0-9]+([.][0-9]+)?')"
    limit="$(printf '%s' "$output" | grep -Eo '"(limit|quota|max)"[[:space:]]*:[[:space:]]*"?[0-9]+([.][0-9]+)?"?' | head -n1 | grep -Eo '[0-9]+([.][0-9]+)?')"
    period="$(printf '%s' "$output" | grep -Eo '"(period|billing_period|window)"[[:space:]]*:[[:space:]]*"[^"]+"' | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]+)"/\1/')"
    unit="$(printf '%s' "$output" | grep -Eo '"(unit|metric)"[[:space:]]*:[[:space:]]*"[^"]+"' | head -n1 | sed -E 's/.*:[[:space:]]*"([^"]+)"/\1/')"

    [[ -z "$used" ]] && used="n/a"
    [[ -z "$limit" ]] && limit="n/a"
    [[ -z "$period" ]] && period="current"
    [[ -z "$unit" ]] && unit="$(detect_unit_from_text "$output")"

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

    token="${CLAUDE_API_TOKEN:-}"
    token="$(extract_json_value_from_file "$credentials_file" "access_token")"
    if [[ -z "$token" ]]; then
        token="$(extract_json_value_from_file "$credentials_file" "accessToken")"
    fi
    if [[ -z "$token" ]]; then
        token="${CLAUDE_API_TOKEN:-}"
    fi
    if [[ -z "$token" ]]; then
        token="$(extract_env_value_from_dotenv "CLAUDE_API_TOKEN")"
    fi

    if [[ -z "$token" ]] && command -v security &> /dev/null; then
        token="$(security find-generic-password -w -s "Claude Code-credentials" 2>/dev/null)"
    fi

    if [[ -z "$token" ]]; then
        append_usage_result "claude-code" "current" "n/a" "n/a" "requests" "oauth" "error" "No Claude OAuth token from credentials/keychain" "experimental_oauth_api"
        return 1
    fi

    local output
    output="$(curl -sS -w $'\n__HTTP_STATUS__:%{http_code}' \
        -H "Authorization: Bearer ${token}" \
        -H "Accept: application/json" \
        -H "Content-Type: application/json" \
        -H "anthropic-beta: oauth-2025-04-20" \
        -H "User-Agent: one-click-tools/experimental" \
        "$endpoint" 2>&1)"
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        local curl_preview
        curl_preview="$(printf '%s' "$output" | tr '\n' ' ' | sed -E 's/[[:space:]]+/ /g' | cut -c1-180)"
        append_usage_result "claude-code" "current" "n/a" "n/a" "requests" "oauth" "error" "Claude experimental API request failed: ${curl_preview}" "experimental_oauth_api"
        return 1
    fi

    local http_status body
    http_status="$(printf '%s' "$output" | sed -n 's/^__HTTP_STATUS__://p' | tail -n1)"
    body="$(printf '%s' "$output" | sed '/^__HTTP_STATUS__:/d')"
    if [[ -z "$http_status" ]]; then
        http_status="000"
    fi
    if [[ "$http_status" -lt 200 || "$http_status" -ge 300 ]]; then
        local err_preview
        err_preview="$(printf '%s' "$body" | tr '\n' ' ' | sed -E 's/[[:space:]]+/ /g' | cut -c1-180)"
        append_usage_result "claude-code" "current" "n/a" "n/a" "requests" "oauth" "error" "Claude experimental API HTTP ${http_status}: ${err_preview}" "experimental_oauth_api"
        return 1
    fi

    local five_hour seven_day
    five_hour="$(python3 - <<'PY' "$body"
import json,sys
raw=sys.argv[1]
try:
    d=json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)
v=((d.get("five_hour") or {}).get("utilization"))
print("" if v is None else v)
PY
)"
    seven_day="$(python3 - <<'PY' "$body"
import json,sys
raw=sys.argv[1]
try:
    d=json.loads(raw)
except Exception:
    print("")
    raise SystemExit(0)
v=((d.get("seven_day") or {}).get("utilization"))
print("" if v is None else v)
PY
)"

    if [[ -z "$five_hour" ]] && [[ -z "$seven_day" ]]; then
        local preview
        preview="$(printf '%s' "$body" | tr '\n' ' ' | sed -E 's/[[:space:]]+/ /g' | cut -c1-180)"
        append_usage_result "claude-code" "current" "n/a" "n/a" "requests" "oauth" "error" "Claude experimental API parse failed: ${preview}" "experimental_oauth_api"
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
    local token_endpoint="https://oauth2.googleapis.com/token"
    local codeassist_base="https://cloudcode-pa.googleapis.com/v1internal"
    local token refresh_token expiry_date now_ms
    local client_id client_secret
    local load_output load_status load_body project_id
    local quota_output quota_status quota_body
    local parse_result used limit unit primary_model secondary_model

    token="$(extract_json_value_from_file "$oauth_file" "access_token")"
    if [[ -z "$token" ]]; then
        token="$(extract_json_value_from_file "$oauth_file" "accessToken")"
    fi
    refresh_token="$(extract_json_value_from_file "$oauth_file" "refresh_token")"
    expiry_date="$(sed -nE 's/.*"expiry_date"[[:space:]]*:[[:space:]]*([0-9]+).*/\1/p' "$oauth_file" | head -n1)"

    if [[ -z "$token" ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "No Gemini OAuth token in ~/.gemini/oauth_creds.json" "experimental_oauth_api"
        return 1
    fi

    if [[ -n "$expiry_date" ]]; then
        now_ms="$(($(date +%s) * 1000))"
        if [[ "$expiry_date" -le $((now_ms + 60000)) ]] && [[ -n "$refresh_token" ]]; then
            client_id="$(python3 - <<'PY'
import os, re
candidates = []
gemini = None
try:
    import subprocess
    out = subprocess.check_output(["/usr/bin/env", "which", "gemini"], stderr=subprocess.DEVNULL).decode().strip()
    if out:
        gemini = os.path.realpath(out)
except Exception:
    pass
if gemini:
    root = os.path.dirname(os.path.dirname(gemini))
    candidates.append(os.path.join(root, "node_modules", "@google", "gemini-cli-core", "dist/src/code_assist/oauth2.js"))
candidates.extend([
    "/opt/homebrew/lib/node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
    "/usr/local/lib/node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
])
for p in candidates:
    if not os.path.exists(p):
        continue
    src = open(p, encoding="utf-8", errors="ignore").read()
    m = re.search(r"const OAUTH_CLIENT_ID = '([^']+)';", src)
    if m:
        print(m.group(1))
        raise SystemExit(0)
PY
)"
            client_secret="$(python3 - <<'PY'
import os, re
candidates = []
gemini = None
try:
    import subprocess
    out = subprocess.check_output(["/usr/bin/env", "which", "gemini"], stderr=subprocess.DEVNULL).decode().strip()
    if out:
        gemini = os.path.realpath(out)
except Exception:
    pass
if gemini:
    root = os.path.dirname(os.path.dirname(gemini))
    candidates.append(os.path.join(root, "node_modules", "@google", "gemini-cli-core", "dist/src/code_assist/oauth2.js"))
candidates.extend([
    "/opt/homebrew/lib/node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
    "/usr/local/lib/node_modules/@google/gemini-cli/node_modules/@google/gemini-cli-core/dist/src/code_assist/oauth2.js",
])
for p in candidates:
    if not os.path.exists(p):
        continue
    src = open(p, encoding="utf-8", errors="ignore").read()
    m = re.search(r"const OAUTH_CLIENT_SECRET = '([^']+)';", src)
    if m:
        print(m.group(1))
        raise SystemExit(0)
PY
)"

            if [[ -n "$client_id" ]] && [[ -n "$client_secret" ]]; then
                local refresh_resp
                refresh_resp="$(curl -sS -X POST "$token_endpoint" \
                    -H "Content-Type: application/x-www-form-urlencoded" \
                    --data-urlencode "client_id=${client_id}" \
                    --data-urlencode "client_secret=${client_secret}" \
                    --data-urlencode "refresh_token=${refresh_token}" \
                    --data-urlencode "grant_type=refresh_token" 2>&1)"
                local refresh_rc=$?
                if [[ $refresh_rc -eq 0 ]]; then
                    local refreshed
                    refreshed="$(printf '%s' "$refresh_resp" | sed -nE 's/.*"access_token"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' | head -n1)"
                    if [[ -n "$refreshed" ]]; then
                        token="$refreshed"
                    fi
                fi
            fi
        fi
    fi

    load_output="$(curl -sS -w $'\n__HTTP_STATUS__:%{http_code}' \
        -X POST "${codeassist_base}:loadCodeAssist" \
        -H "Authorization: Bearer ${token}" \
        -H "Accept: application/json" \
        -H "Content-Type: application/json" \
        -d '{"metadata":{"ideType":"IDE_UNSPECIFIED","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}' 2>&1)"
    if [[ $? -ne 0 ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "Gemini loadCodeAssist request failed" "experimental_oauth_api"
        return 1
    fi
    load_status="$(printf '%s' "$load_output" | sed -n 's/^__HTTP_STATUS__://p' | tail -n1)"
    load_body="$(printf '%s' "$load_output" | sed '/^__HTTP_STATUS__:/d')"
    if [[ -z "$load_status" ]]; then load_status="000"; fi
    if [[ "$load_status" -lt 200 || "$load_status" -ge 300 ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "Gemini loadCodeAssist HTTP ${load_status}" "experimental_oauth_api"
        return 1
    fi

    project_id="$(python3 - <<'PY' "$load_body"
import json, sys
try:
    d = json.loads(sys.argv[1])
except Exception:
    print("")
    raise SystemExit(0)
print(d.get("cloudaicompanionProject",""))
PY
)"
    if [[ -z "$project_id" ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "Gemini loadCodeAssist parse failed (no project id)" "experimental_oauth_api"
        return 1
    fi

    quota_output="$(curl -sS -w $'\n__HTTP_STATUS__:%{http_code}' \
        -X POST "${codeassist_base}:retrieveUserQuota" \
        -H "Authorization: Bearer ${token}" \
        -H "Accept: application/json" \
        -H "Content-Type: application/json" \
        -d "{\"project\":\"${project_id}\"}" 2>&1)"
    if [[ $? -ne 0 ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "Gemini retrieveUserQuota request failed" "experimental_oauth_api"
        return 1
    fi
    quota_status="$(printf '%s' "$quota_output" | sed -n 's/^__HTTP_STATUS__://p' | tail -n1)"
    quota_body="$(printf '%s' "$quota_output" | sed '/^__HTTP_STATUS__:/d')"
    if [[ -z "$quota_status" ]]; then quota_status="000"; fi
    if [[ "$quota_status" -lt 200 || "$quota_status" -ge 300 ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "Gemini retrieveUserQuota HTTP ${quota_status}" "experimental_oauth_api"
        return 1
    fi

    parse_result="$(python3 - <<'PY' "$quota_body"
import json, sys, math
try:
    d = json.loads(sys.argv[1])
except Exception:
    print("")
    raise SystemExit(0)
buckets = d.get("buckets") or []
primary_ids = {"gemini-2.5-pro","gemini-3.1-pro-preview","gemini-3-pro-preview"}
secondary_ids = {"gemini-2.5-flash","gemini-2.5-flash-lite","gemini-3-flash-preview","gemini-3.1-flash-lite-preview"}
def pick(ids):
    for b in buckets:
        if b.get("modelId") in ids and b.get("remainingFraction") is not None:
            return b
    for b in buckets:
        if b.get("tokenType") == "REQUESTS" and b.get("remainingFraction") is not None:
            return b
    return None
pb = pick(primary_ids); sb = pick(secondary_ids)
if not pb and not sb:
    print("")
    raise SystemExit(0)
target = pb or sb
rem = target.get("remainingFraction")
used = int(max(0, min(100, round((1-float(rem))*100)))) if rem is not None else 0
print(f"{used}|100|percent|{(pb or {}).get('modelId','n/a')}|{(sb or {}).get('modelId','n/a')}")
PY
)"
    if [[ -z "$parse_result" ]]; then
        append_usage_result "gemini" "current" "n/a" "n/a" "requests" "oauth" "error" "Gemini retrieveUserQuota parse failed" "experimental_oauth_api"
        return 1
    fi

    IFS='|' read -r used limit unit primary_model secondary_model <<< "$parse_result"
    append_usage_result "gemini" "current" "$used" "$limit" "$unit" "oauth" "ok" "Gemini quota fetched (Pro/Flash buckets)" "experimental_oauth_api primary=${primary_model} secondary=${secondary_model}"
    return 0
}

fetch_usage_copilot_api() {
    local endpoint="$1"
    local token="$2"
    local output rc http_status body parse_result used unit period

    output="$(curl -sS -w $'\n__HTTP_STATUS__:%{http_code}' \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer ${token}" \
        -H "X-GitHub-Api-Version: 2026-03-10" \
        "$endpoint" 2>&1)"
    rc=$?
    if [[ $rc -ne 0 ]]; then
        local preview
        preview="$(printf '%s' "$output" | tr '\n' ' ' | sed -E 's/[[:space:]]+/ /g' | cut -c1-180)"
        append_usage_result "copilot" "current" "n/a" "n/a" "requests" "api" "error" "Copilot API request failed: ${preview}"
        return 1
    fi

    http_status="$(printf '%s' "$output" | sed -n 's/^__HTTP_STATUS__://p' | tail -n1)"
    body="$(printf '%s' "$output" | sed '/^__HTTP_STATUS__:/d')"
    if [[ -z "$http_status" ]]; then
        http_status="000"
    fi
    if [[ "$http_status" -lt 200 || "$http_status" -ge 300 ]]; then
        local err_preview
        err_preview="$(printf '%s' "$body" | tr '\n' ' ' | sed -E 's/[[:space:]]+/ /g' | cut -c1-180)"
        append_usage_result "copilot" "current" "n/a" "n/a" "requests" "api" "error" "Copilot API HTTP ${http_status}: ${err_preview}"
        if [[ "$http_status" == "401" ]]; then
            return 2
        fi
        return 1
    fi

    parse_result="$(python3 - <<'PY' "$body"
import json, sys
try:
    data = json.loads(sys.argv[1])
except Exception:
    print("")
    raise SystemExit(0)

items = data.get("usageItems") or []
copilot_items = []
for it in items:
    product = str(it.get("product", "")).lower()
    sku = str(it.get("sku", "")).lower()
    if "copilot" in product or "copilot" in sku:
        copilot_items.append(it)

if not copilot_items:
    print("")
    raise SystemExit(0)

used = 0.0
unit = "requests"
for it in copilot_items:
    q = it.get("netQuantity")
    if q is None:
        q = it.get("grossQuantity")
    if q is None:
        q = it.get("quantity")
    if q is None:
        continue
    try:
        used += float(q)
    except Exception:
        pass
    if it.get("unitType"):
        unit = str(it.get("unitType"))

period_obj = data.get("timePeriod") or {}
year = period_obj.get("year")
month = period_obj.get("month")
day = period_obj.get("day")
if year and month and day:
    period = f"{year}-{int(month):02d}-{int(day):02d}"
elif year and month:
    period = f"{year}-{int(month):02d}"
elif year:
    period = str(year)
else:
    period = "current"

used_out = int(used) if float(used).is_integer() else round(used, 2)
print(f"{used_out}|{unit}|{period}")
PY
)"

    if [[ -z "$parse_result" ]]; then
        append_usage_result "copilot" "current" "n/a" "n/a" "requests" "api" "error" "Copilot API parse failed (no copilot usageItems)"
        return 1
    fi

    IFS='|' read -r used unit period <<< "$parse_result"
    append_usage_result "copilot" "$period" "$used" "n/a" "$unit" "api" "ok" "GitHub billing premium_request usage fetched"
    return 0
}

fetch_github_login_from_api() {
    local token="$1"
    local output http_status body
    output="$(curl -sS -w $'\n__HTTP_STATUS__:%{http_code}' \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer ${token}" \
        -H "X-GitHub-Api-Version: 2026-03-10" \
        "https://api.github.com/user" 2>/dev/null)" || return 1
    http_status="$(printf '%s' "$output" | sed -n 's/^__HTTP_STATUS__://p' | tail -n1)"
    body="$(printf '%s' "$output" | sed '/^__HTTP_STATUS__:/d')"
    if [[ -z "$http_status" ]] || [[ "$http_status" -lt 200 || "$http_status" -ge 300 ]]; then
        return 1
    fi
    python3 - <<'PY' "$body"
import json, sys
try:
    d = json.loads(sys.argv[1])
except Exception:
    raise SystemExit(1)
login = d.get("login") or ""
if login:
    print(login)
    raise SystemExit(0)
raise SystemExit(1)
PY
}

fetch_github_login_from_gh() {
    if ! command -v gh &> /dev/null; then
        return 1
    fi
    gh api user --jq '.login' 2>/dev/null | head -n1
}

fetch_github_login_from_gh_status() {
    if ! command -v gh &> /dev/null; then
        return 1
    fi
    gh auth status 2>/dev/null | sed -nE 's/.*Logged in to [^ ]+ as ([^ ]+).*/\1/p' | head -n1
}

fetch_github_org_from_gh() {
    if ! command -v gh &> /dev/null; then
        return 1
    fi
    gh api user/orgs --jq '.[0].login' 2>/dev/null | head -n1
}

fetch_github_orgs_from_gh() {
    if ! command -v gh &> /dev/null; then
        return 1
    fi
    gh api user/orgs --jq '.[].login' 2>/dev/null
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

get_usage_copilot() {
    local api_key="${GITHUB_TOKEN:-${GH_TOKEN:-${GITHUB_API_TOKEN:-${COPILOT_API_KEY:-}}}}"
    local endpoint="${OCT_COPILOT_USAGE_ENDPOINT:-}"
    local enterprise="${OCT_GITHUB_ENTERPRISE:-${GITHUB_ENTERPRISE:-}}"
    local org="${OCT_GITHUB_ORG:-${GITHUB_ORG:-}}"
    local user_name="${OCT_GITHUB_USER:-${GITHUB_USER:-}}"
    local attempted_api=0
    local year="${OCT_COPILOT_USAGE_YEAR:-}"
    local month="${OCT_COPILOT_USAGE_MONTH:-}"
    local day="${OCT_COPILOT_USAGE_DAY:-}"
    local model="${OCT_COPILOT_USAGE_MODEL:-}"
    local product="${OCT_COPILOT_USAGE_PRODUCT:-}"
    local query=""
    local -a token_candidates=()
    local -a org_candidates=()
    local token_rc=1

    add_token_candidate "${GITHUB_API_TOKEN:-}"
    local t
    t="$(extract_env_value_from_dotenv "GITHUB_API_TOKEN")"
    add_token_candidate "$t"
    if [[ -z "$endpoint" ]]; then
        endpoint="$(extract_env_value_from_dotenv "OCT_COPILOT_USAGE_ENDPOINT")"
    fi
    # User-only mode for Copilot usage lookup: intentionally ignore org/enterprise paths.
    org=""
    enterprise=""
    if [[ -z "$user_name" ]]; then
        user_name="$(extract_env_value_from_dotenv "OCT_GITHUB_USER")"
    fi
    if [[ -z "$user_name" ]]; then
        user_name="$(extract_env_value_from_dotenv "GITHUB_USER")"
    fi
    if [[ -z "$year" ]]; then
        year="$(extract_env_value_from_dotenv "OCT_COPILOT_USAGE_YEAR")"
    fi
    if [[ -z "$month" ]]; then
        month="$(extract_env_value_from_dotenv "OCT_COPILOT_USAGE_MONTH")"
    fi
    if [[ -z "$day" ]]; then
        day="$(extract_env_value_from_dotenv "OCT_COPILOT_USAGE_DAY")"
    fi
    if [[ -z "$model" ]]; then
        model="$(extract_env_value_from_dotenv "OCT_COPILOT_USAGE_MODEL")"
    fi
    if [[ -z "$product" ]]; then
        product="$(extract_env_value_from_dotenv "OCT_COPILOT_USAGE_PRODUCT")"
    fi

    [[ -n "$year" ]] && query="${query}&year=${year}"
    [[ -n "$month" ]] && query="${query}&month=${month}"
    [[ -n "$day" ]] && query="${query}&day=${day}"
    [[ -n "$model" ]] && query="${query}&model=${model}"
    [[ -n "$product" ]] && query="${query}&product=${product}"
    if [[ -n "$query" ]]; then
        query="?${query#&}"
    fi

    if [[ ${#token_candidates[@]} -eq 0 ]]; then
        append_usage_result "copilot" "current" "n/a" "n/a" "requests" "api" "error" "No GitHub token set (GITHUB_API_TOKEN)"
        return 1
    fi

    for api_key in "${token_candidates[@]}"; do
        if [[ -n "$endpoint" ]]; then
            attempted_api=1
            fetch_usage_copilot_api "$endpoint" "$api_key"
            token_rc=$?
            if [[ $token_rc -eq 0 ]]; then return 0; fi
            [[ $token_rc -eq 2 ]] && continue
        fi

        if [[ -n "$user_name" ]]; then
            endpoint="https://api.github.com/users/${user_name}/settings/billing/premium_request/usage${query}"
            attempted_api=1
            fetch_usage_copilot_api "$endpoint" "$api_key"
            token_rc=$?
            if [[ $token_rc -eq 0 ]]; then return 0; fi
            [[ $token_rc -eq 2 ]] && continue
        fi

        if [[ -z "$user_name" ]]; then
            user_name="$(fetch_github_login_from_api "$api_key" 2>/dev/null || true)"
            if [[ -z "$user_name" ]]; then
                user_name="$(fetch_github_login_from_gh 2>/dev/null || true)"
            fi
            if [[ -z "$user_name" ]]; then
                user_name="$(fetch_github_login_from_gh_status 2>/dev/null || true)"
            fi
            if [[ -n "$user_name" ]]; then
                endpoint="https://api.github.com/users/${user_name}/settings/billing/premium_request/usage${query}"
                attempted_api=1
                fetch_usage_copilot_api "$endpoint" "$api_key"
                token_rc=$?
                if [[ $token_rc -eq 0 ]]; then return 0; fi
                [[ $token_rc -eq 2 ]] && continue
            fi
        fi

    done

    if [[ $attempted_api -eq 1 ]]; then
        append_usage_result "copilot" "current" "n/a" "n/a" "requests" "api" "error" "Copilot user API failed. Fine-grained PAT needs Plan(read)."
    fi

    if [[ -n "$api_key" ]] && [[ $attempted_api -eq 0 ]]; then
        append_usage_result "copilot" "current" "n/a" "n/a" "requests" "api" "error" "Copilot API skipped: set OCT_COPILOT_USAGE_ENDPOINT or OCT_GITHUB_USER"
    fi

    return 1
}

get_usage_all() {
    local output_mode="${1:-table}"
    local ok_count=0
    USAGE_RESULTS=()

    get_usage_codex && ok_count=$((ok_count + 1))
    get_usage_claude && ok_count=$((ok_count + 1))
    get_usage_gemini && ok_count=$((ok_count + 1))
    get_usage_copilot && ok_count=$((ok_count + 1))

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
