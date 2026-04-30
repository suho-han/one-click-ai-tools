# 로컬 테스트 가이드

## 1. 스크립트 직접 실행 (가장 빠름)

npm 설치 없이 스크립트를 직접 호출해 동작을 확인한다.

```bash
# 도움말
bash scripts/update-ai-cli.sh help

# 잘못된 명령어 처리 확인
bash scripts/update-ai-cli.sh unknown-cmd

# AI 도구 업데이트 (실제 brew/npm 호출됨 — 주의)
bash scripts/update-ai-cli.sh agent-update

# oct 자체 업데이트 (실제 npm install 호출됨 — 주의)
bash scripts/update-ai-cli.sh update
bash scripts/update-ai-cli.sh update --beta
```

## 2. npm link로 `oct` 명령어 테스트

로컬 소스를 전역 `oct` 명령어로 등록해 실제 설치 환경과 동일하게 테스트한다.

```bash
# 프로젝트 루트에서 실행
npm link

# 이후 oct 명령어로 테스트
oct help
oct agent-update
oct update --beta

# 테스트 완료 후 해제
npm unlink -g one-click-tools
```

## 3. 문법 검사

코드 변경 후 Bash 문법 오류를 빠르게 확인한다.

```bash
# 메인 파일
bash -n scripts/update-ai-cli.sh

# 모든 lib 파일
for f in scripts/lib/*.sh; do bash -n "$f" && echo "OK: $f"; done
```

## 4. 로그 파일 확인

`agent-update` 실행 시 `~/.oct/logs/`에 로그가 생성된다.

```bash
# 최근 로그 확인
ls -lt ~/.oct/logs/
cat ~/.oct/logs/$(ls -t ~/.oct/logs/ | head -1)
```

## 5. 드라이런 — 실제 설치 없이 분기 확인

특정 도구가 설치되어 있지 않은 상황을 시뮬레이션하려면 PATH를 조작한다.

```bash
# brew, npm 명령어를 숨겨서 fallback 경로 테스트
PATH_ORIG=$PATH

# npm 없는 환경 시뮬레이션
PATH=$(echo "$PATH" | tr ':' '\n' | grep -v "$(dirname $(which npm))" | tr '\n' ':') \
  bash scripts/update-ai-cli.sh agent-update

PATH=$PATH_ORIG
```

## 주의사항

- `agent-update`는 실제로 brew/npm을 실행하므로 패키지가 업데이트될 수 있다.
- `update` / `update --beta`는 실제로 `one-click-tools` npm 패키지를 재설치한다.
- 안전하게 확인하려면 **섹션 1의 `help`** 또는 **섹션 3의 문법 검사**부터 시작한다.
