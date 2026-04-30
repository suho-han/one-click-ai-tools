# PLAN

## 현재 상태 (v0.2.4)

```
oct agent-update        — claude/codex/gemini/copilot 전체 업데이트
oct update [--beta]     — oct 자체 업데이트
oct usage [--json]      — 사용량 조회
oct help                — 도움말
```

---

## Feature 1: Config (업데이트 대상 기본값 설정)

### 목표
`~/.oct/config` 파일로 기본 업데이트 대상 도구를 선택할 수 있게 한다.

### 저장 위치
`~/.oct/config` (key=value 형식, 파싱이 단순한 shell-native 형식)

```ini
enabled_tools=claude,codex,gemini,copilot
```

### 신규 파일
- `scripts/lib/config-manager.sh` — config 파일 읽기/쓰기/초기화

### 변경 파일
- `scripts/lib/update-macos.sh`, `scripts/lib/update-ubuntu.sh` — enabled_tools 필터링
- `scripts/lib/help.sh` — `oct config` 커맨드 안내 추가
- `scripts/update-ai-cli.sh` — `config` 커맨드 추가

### 신규 커맨드
```
oct config                        — 현재 설정 표시
oct config set tools claude,codex — 업데이트 대상 변경
oct config reset                  — 기본값으로 초기화
```

### 구현 순서
1. `config-manager.sh` 작성 (load_config, save_config, show_config, set_config)
2. `update-macos.sh` / `update-ubuntu.sh` 에서 TOOLS 순회 전 enabled_tools 필터 적용
3. dispatcher에 `config` 커맨드 추가

---

## Feature 2: Schedule (자동 업데이트 스케줄)

### 목표
`oct agent-update`를 주기적으로 자동 실행하는 스케줄을 등록/해제할 수 있게 한다.

### 구현 방식
| OS | 메커니즘 |
|----|---------|
| macOS | launchd plist (`~/Library/LaunchAgents/com.oct.agent-update.plist`) |
| Linux | crontab entry |

### Config 저장 위치
`~/.oct/config` 에 스케줄 설정 추가

```ini
schedule_enabled=true
schedule_interval=daily    # daily | weekly | off
schedule_tools=claude,codex
schedule_hour=9            # 실행 시각 (시)
```

### 신규 파일
- `scripts/lib/schedule.sh` — enable_schedule, disable_schedule, show_schedule_status

### 신규 커맨드
```
oct schedule                       — 현재 스케줄 상태 확인
oct schedule enable [--daily|--weekly] [--hour 9] [--tools claude,codex]
oct schedule disable               — 스케줄 해제
```

### launchd plist 구조 (macOS)
```xml
<key>StartCalendarInterval</key>
<dict>
    <key>Hour</key><integer>9</integer>
    <key>Minute</key><integer>0</integer>
    <!-- weekly: + <key>Weekday</key><integer>1</integer> -->
</dict>
```

### 구현 순서
1. `schedule.sh` 작성
2. macOS launchd 경로: `generate_plist()` → `launchctl load`
3. Linux cron: `crontab -l` + append + `crontab -`
4. `schedule disable`: `launchctl unload` + plist 삭제 (macOS), `crontab -l | grep -v oct` (Linux)
5. dispatcher에 `schedule` 커맨드 추가

---

## Feature 3: UI 개선

### 목표
출력 가독성 향상 — 진행 상황과 결과를 더 명확하게 표시한다.

### 개선 항목
| 항목 | 현재 | 개선 |
|------|------|------|
| 업데이트 진행 | 텍스트 로그 | `[1/4] Claude Code...` 형식 카운터 |
| 결과 요약 | 단순 목록 | 성공/실패 아이콘 + 소요 시간 |
| config 표시 | 없음 | 박스 형태 설정 화면 |
| 스케줄 상태 | 없음 | 다음 실행 시각 표시 |

### 구체적 변경
- `results.sh` — `summarize_results()`에 이모지/아이콘 추가 옵션
- `update-macos.sh` / `update-ubuntu.sh` — `[N/total]` 카운터 출력
- `help.sh` — 신규 커맨드 반영

---

## 구현 우선순위

1. **Feature 1 (Config)** — 가장 작고 독립적, 다른 feature의 기반
2. **Feature 3 (UI)** — Feature 1과 동시 진행 가능
3. **Feature 2 (Schedule)** — Config 구조에 의존, 마지막

---

## 검증 방법

```bash
# Config
oct config
oct config set tools claude,codex
oct agent-update   # codex, claude만 업데이트되는지 확인
oct config reset

# Schedule (macOS)
oct schedule enable --daily --hour 9
launchctl list | grep oct
oct schedule
oct schedule disable
launchctl list | grep oct   # 사라졌는지 확인

# UI
oct agent-update   # [1/4], [2/4]... 카운터 확인
```
