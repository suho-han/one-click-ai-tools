# 로컬 테스트 가이드

## 빠른 실행

```bash
go run main.go help
go run main.go usage --json
```

주의:
- `go run main.go agent-update`는 실제 패키지 업데이트를 실행합니다.

## 빌드/실행

```bash
go build -o oct main.go
./oct help
./oct usage
```

## 테스트

```bash
GOTOOLCHAIN=auto go test ./...
GOTOOLCHAIN=auto go test -cover ./...
```

## npm 래퍼 확인

```bash
go build -o oct main.go
npm link
oct help
oct usage
npm unlink -g one-click-tools
```

## API Mock/엔드포인트 테스트

```bash
OCT_CLAUDE_USAGE_ENDPOINT="http://localhost:8080/usage" go run main.go usage
```

## Windows 검증 핵심

```powershell
go build -o oct.exe main.go
.\oct.exe help
.\oct.exe usage --json
```

체크 포인트:
- 명령 실행/출력 정상
- 경로(공백 포함)에서 실행 정상
- `schedule enable/disable` 및 Task Scheduler 등록 확인
