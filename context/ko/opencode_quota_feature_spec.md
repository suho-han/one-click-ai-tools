# OpenCode Go Quota 모니터링 및 비용 시뮬레이터 개발 스펙

본 문서는 `one-click-ai-tools` 프로젝트 내에 OpenCode Go의 실시간 쿼터 잔량 조회 및 캐시 히트율 기반 비용 시뮬레이터 기능을 원클릭 CLI 명령어로 제공하기 위한 개발 스펙 및 기획서임.

---

## 1. 개요 및 배경

* **배경**: 코딩 에이전트 구동 시 컨텍스트가 70K~120K에 달해 쿼터 소진 속도가 매우 빠름. OpenCode Go의 쿼터 제한(5시간 $12, 주간 $30, 월간 $60)을 사용자가 터미널에서 즉시 파악하고 대응할 수 있도록 지원할 필요가 있음.
* **목표**: `oct quota` 명령어를 추가하여 현재 쿼터 소진량 확인, 리셋 카운트다운 조회, 로컬 사용 이력을 바탕으로 한 최적의 모델(MiMo-V2.5 등) 비용 시뮬레이션 기능을 일괄 제공함.

---

## 2. 요구사항 정의 (Requirements)

### 2.1 인증 및 설정 감지
* **환경 변수 감지**: 터미널의 `OPENCODE_API_KEY` 환경 변수를 자동 감지하고, 없을 경우 `~/.zshrc` 또는 `~/.env` 파일에서 우선 파싱하여 수집함.
* **설정 파일 동기화**: `~/.config/opencode/opencode.json` 파일의 `provider.opencode-go` 설정 여부를 진단하고 계정 연동 상태를 확인해줌.

### 2.2 실시간 Quota 모니터링 (TUI)
* **쿼터 데이터 조회**: OpenCode Go API 엔드포인트(`https://opencode.ai/zen/go/v1` 혹은 `opencode-quota` 연동 API)를 쿼리하여 남은 한도 잔량을 회신받음.
* **프로그레스 바 렌더링**: Rolling 5-Hour ($12), Weekly ($30), Monthly ($60)의 잔여 백분율을 터미널 UI 프로그레스 바(`████░░░░ 50% left`)로 렌더링함.
* **리셋 Countdown**: Rolling 리셋까지 남은 시간(예: 약 3시간 42분 등)을 시각적으로 표시함.

### 2.3 캐시 히트율 기반 비용 시뮬레이터
* **로컬 통계 파싱**: 로컬 세션 데이터(Input, Output, Cache Read 토큰 수)를 파싱하여 현재 세션의 캐시 히트율(`Cache Read / (Input + Cache Read)`)을 계산함.
* **시나리오 비교 연산**:
  * **DeepSeek V4 Pro**: `Input $0.66/1M`, `Hit $0.022/1M`, `Output $1.98/1M`
  * **Qwen3.7 Plus**: `Input $0.40/1M`, `Hit $0.080/1M`, `Output $1.60/1M`
  * **MiMo-V2.5**: `Input $0.14/1M`, `Hit $0.0028/1M`, `Output $0.28/1M`
  위 단가를 대입하여 소모된 예상 달러 비용을 상호 비교해줌.
* **최적 모델 자동 추천**: 
  * 캐시 히트율이 90% 이상으로 극단적으로 높은 에이전트 세션의 경우, 캐시 히트 단가가 가장 저렴한 **MiMo-V2.5**를 최우선 추천함.
  * 캐시 히트율이 낮거나 단발성인 경우 **Qwen3.7 Plus** 또는 **MiniMax M3**를 추천함.

---

## 3. Go 프로젝트 아키텍처 및 구현 방향

### 3.1 CLI 명령어 등록
* 파일 경로: `cmd/quota.go`
* `oct quota` 명령어를 통해 서브 커맨드 형태로 기능 진입.
  * `oct quota show`: 실시간 쿼터 바 렌더링.
  * `oct quota stats`: 로컬 세션 요약 및 비용 시뮬레이터 출력.

### 3.2 내부 로직 구조 (Internal Package)
* **`internal/quota/client.go`**: API 인증 및 HTTP 통신 모듈.
* **`internal/quota/parser.go`**: 로컬 세션 토큰 로그 분석 및 캐시 히트율 연산 모듈.
* **`internal/quota/simulator.go`**: 모델별 단가를 대입하여 시나리오별 예상 과금액 산출 및 추천 엔진 모듈.

---

## 4. UI/UX 기대 예시 출력

```bash
$ oct quota show
[OpenCode Go Quota Status]
- Rolling 5-Hour: ██████████████████████░░░░░░░░░░░░░░░░░ 57% left (Reset in 3h 12m)
- Weekly Cap:     ██████████████████████████████████░░░░ 87% left (Reset in 2d)
- Monthly Cap:    ████████████████████████████████████░░ 94% left (Reset in 14d)

$ oct quota stats
[Local Session Stats - Last 17 Days]
- Cache Miss Input: 33.1M tokens
- Cache Hit Input:  742.5M tokens
- Output Generated: 1.8M tokens
- Actual Cache Hit Rate: 95.7%

[Estimated Cost Comparison]
- DeepSeek V4 Pro: $41.75
- Qwen3.7 Plus:    $75.52 (High cache-hit penalty)
- MiMo-V2.5:       $7.22  (Highly Recommended! Save 82.7%)
```

---
*참고: 본 기능은 Z.ai 및 OpenCode Go API 연동 스펙 변경에 유연하게 대응할 수 있도록 `baseURL`과 `coefficients`를 CLI 플래그나 설정 파일에서 오버라이드할 수 있도록 개발해야 함.*
