# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

### [0.4.15](https://github.com/suho-han/one-click-tools/compare/v0.4.14...v0.4.15) (2026-06-12)

### [0.4.14](https://github.com/suho-han/one-click-tools/compare/v0.4.13...v0.4.14) (2026-06-10)


### Features

* **session-refresh:** add claude and opencode probes ([0f1bbdb](https://github.com/suho-han/one-click-tools/commit/0f1bbdbbb08cf3c34df340fcbf7ebaacc41ceb0a))
* **session-refresh:** add confidence to probe results ([bb6b327](https://github.com/suho-han/one-click-tools/commit/bb6b3276494d944ec93e4b4f9f78ba51f1078b04))


### Bug Fixes

* **monitor:** preserve configured provider order and warn severity ([bb76f03](https://github.com/suho-han/one-click-tools/commit/bb76f033a685f3996796d05e33b173df58ad04b0))
* **session-refresh:** bootstrap CLI lookup via execenv ([4417bc1](https://github.com/suho-han/one-click-tools/commit/4417bc159e39871277f4b9064269cf24c9680272))
* **update:** avoid brew misdetection and serialize brew upgrades ([e5a19c3](https://github.com/suho-han/one-click-tools/commit/e5a19c3995544ca24ade9ef40bdb766eeb5fa07d))
* **update:** bootstrap tool paths for non-interactive shells ([0bd6eb9](https://github.com/suho-han/one-click-tools/commit/0bd6eb90ba4aa76567c715f560564e00549341a2))
* **update:** support brew copilot-cli ownership ([add4ca1](https://github.com/suho-han/one-click-tools/commit/add4ca176856b83056b9c60666bc47d37246e7d8))


### Refactors

* **exec:** share bootstrapped command env helpers ([6d1e2a6](https://github.com/suho-han/one-click-tools/commit/6d1e2a6394828e55a9d7cb3b19acf59ddc3d6077))

### [0.4.13](https://github.com/suho-han/one-click-tools/compare/v0.4.12...v0.4.13) (2026-06-10)


### Features

* **config:** add session refresh defaults ([836b8d0](https://github.com/suho-han/one-click-tools/commit/836b8d093865b67a93c2a14083e780ff735a3ee7))
* **install:** prompt for session refresh scheduling ([015293d](https://github.com/suho-han/one-click-tools/commit/015293d78a0f723851f28954e6a1e81f06589669))
* **session-refresh:** add scheduled token-free session probes ([01e8078](https://github.com/suho-han/one-click-tools/commit/01e80781f19e3f5b64d37200987c4dcf69263f58))
* **session-refresh:** include refreshed usage and ag alias support ([5b8c0cc](https://github.com/suho-han/one-click-tools/commit/5b8c0cc57efb8da9a76c2c8c04eb331c8de1b9f4))
* **update:** use official Cursor CLI install flow ([3802989](https://github.com/suho-han/one-click-tools/commit/3802989cb3b8632056de9cc855c63cce3d118492))
* **usage:** migrate gemini provider to antigravity ([2588e42](https://github.com/suho-han/one-click-tools/commit/2588e4230225e1e21133abd48d1ff44533cba843))


### Bug Fixes

* **antigravity:** require official agy binary only ([2a58878](https://github.com/suho-han/one-click-tools/commit/2a5887892937dc1ff66501d0b89c39e3f8c3dfef))
* **schedule:** prefer current oct binary for scheduled tasks ([184d206](https://github.com/suho-han/one-click-tools/commit/184d206efe0b84bacbf7295d6bad2b08f0564583))
* **test:** harden smoke-matrix cross-platform regressions ([3fec3b0](https://github.com/suho-han/one-click-tools/commit/3fec3b0fe6b6b5aca0fc8723cfcb4306c3dbce1c))
* **update:** add manager support matrix for antigravity ([7b4f62d](https://github.com/suho-han/one-click-tools/commit/7b4f62ddaed4b75cc23da57ca69252f488f9741d))
* **update:** prefer provenance-first manager detection ([77bc794](https://github.com/suho-han/one-click-tools/commit/77bc794e083c546b9563cbb00c5db8e3e78724c0))
* **usage:** clarify cursor and opencode fallback states ([4d65243](https://github.com/suho-han/one-click-tools/commit/4d652434a776e8ac90c345d5f9cf4769d84355d5))


### Documentation

* add antigravity migration and session refresh plan ([6ae8e66](https://github.com/suho-han/one-click-tools/commit/6ae8e6678bb884795dd4a5dd4affcfd24c796e52))
* add package manager stability plan ([ab96ce1](https://github.com/suho-han/one-click-tools/commit/ab96ce187908765dff8e30c9cdff5e62c5e40528))
* canonicalize antigravity naming ([26b6d86](https://github.com/suho-han/one-click-tools/commit/26b6d86206bf3a2deee4f762b82c7be382e180ea))
* **plan:** add antigravity cleanup and usage quality plans ([32332d8](https://github.com/suho-han/one-click-tools/commit/32332d8282df75f7c50336bac3c714642f203de7))
* **plan:** expand scheduler e2e validation checklist ([1fc2835](https://github.com/suho-han/one-click-tools/commit/1fc28355590dd1e27c799068f55a218857f9b4fa))
* **usage:** finish antigravity naming cleanup ([327c409](https://github.com/suho-han/one-click-tools/commit/327c4091dba0593a337e7ff05e0983e403b53adf))

### [0.4.3](https://github.com/suho-han/one-click-tools/compare/v0.4.1...v0.4.3) (2026-05-12)


### Features

* **alert:** add interactive provider selector for provider threshold ([7510e7a](https://github.com/suho-han/one-click-tools/commit/7510e7a12408d91f2df893f20631aa33c93606a3))
* **alert:** add priority-based labeling for usage notifications ([7657ebb](https://github.com/suho-han/one-click-tools/commit/7657ebbfb7355028a7ea56e7f69af93f7109da3f))
* **alert:** show priority in test output and cleanup expired snoozes ([ecd383c](https://github.com/suho-han/one-click-tools/commit/ecd383cad3c9f562ba80645981df9d357f68b14a))
* **alert:** support direct provider/window threshold config keys ([eb23317](https://github.com/suho-han/one-click-tools/commit/eb23317050499d26f5daba0dd485073177de80d5))
* **cli:** order commands by usage frequency and group by function ([67b407f](https://github.com/suho-han/one-click-tools/commit/67b407f92ad17bf77338b1b5534882d8b6b3b368))
* **cli:** split agent-update and oct self-update commands ([51af0df](https://github.com/suho-han/one-click-tools/commit/51af0df924a7f69e71f46a7c0178efddb0cb574c))
* complete p1-p4 usage/update/schedule improvements ([d2224dc](https://github.com/suho-han/one-click-tools/commit/d2224dcc9b77cb27cad84b39cab5cca8944f0bf5))
* **cursor:** add cursor-agent update and usage support ([ff511e8](https://github.com/suho-han/one-click-tools/commit/ff511e83911cce70d9afe15802341a70156d5adc))
* **monitor-alert:** implement P6 detailed alerts and P7 always-on monitor ([2a1641f](https://github.com/suho-han/one-click-tools/commit/2a1641ff871efedddac17cc06f27304e3901fcf7))
* **p8:** enhance monitor output and add alert snooze rules ([5c538e2](https://github.com/suho-han/one-click-tools/commit/5c538e21c6ecce6d0bdc7c79eb6e85cb346d08a6))
* **ui:** add Cursor/OpenCode icons and hide icons on unsupported terminals ([f08a2bd](https://github.com/suho-han/one-click-tools/commit/f08a2bd11e42920ad5d2503215d3f6a4b1a64ea9))
* **usage:** improve Cursor usage retrieval via local auth API with fallback ([52d9cf9](https://github.com/suho-han/one-click-tools/commit/52d9cf93cc3ec0722d4fa52465e4cd704ba70d48))
* **usage:** improve dark-terminal readability for usage and monitor ([ae7d850](https://github.com/suho-han/one-click-tools/commit/ae7d8506b9c49e05cc937e9dc89944c91ec848d0))
* **usage:** optimize terminal layout and compact json output ([f357f56](https://github.com/suho-han/one-click-tools/commit/f357f560ad712b1ced63d1c15af6ace2ecda7011))


### Bug Fixes

* **monitor:** align columns correctly with ANSI-colored labels ([0955a38](https://github.com/suho-han/one-click-tools/commit/0955a384328affee9b793dd714cabc43fbcc5926))
* **update:** recover from npm install conflicts ([09286b9](https://github.com/suho-han/one-click-tools/commit/09286b997ccc2584c5509b1349316fa9163e062f))
* **usage:** respect enabled_tools when selecting providers ([5faee6c](https://github.com/suho-han/one-click-tools/commit/5faee6cb7e3832604b1ddf72f8b4702c7db94fc9))


### Refactors

* **update:** apply code review improvements ([a076913](https://github.com/suho-han/one-click-tools/commit/a0769132fa655bf6d097271cc168c4195e8c1d54))
* **usage:** centralize selected tool resolution ([fddcb08](https://github.com/suho-han/one-click-tools/commit/fddcb0847f6c9489c7972529975f4bec18662d0a))


### Documentation

* add p5 implementation plan ([ae53e8b](https://github.com/suho-han/one-click-tools/commit/ae53e8b150e63b25afa0477990c61168712f718c))
* **alert:** document priority labels and quiet-hours critical override ([658efa0](https://github.com/suho-han/one-click-tools/commit/658efa010109629ca59f34a93232e33bc8f914c1))
* **alert:** note priority field in alert test command output ([1cd051e](https://github.com/suho-han/one-click-tools/commit/1cd051ea17fe49d9d9c7f7fdedf5791b4dcae896))
* **alert:** update provider threshold usage and supported providers ([8e0b26c](https://github.com/suho-han/one-click-tools/commit/8e0b26c6c358026d98301489d8459b91b73ae098))
* **go:** declare minimum Go 1.22 and remove unsupported toolchain directive ([085c61f](https://github.com/suho-han/one-click-tools/commit/085c61f981f3b605a5585a9afb10b0683127a4bf))
* **icons:** update Cursor/OpenCode icon mapping notes ([3a66b3f](https://github.com/suho-han/one-click-tools/commit/3a66b3f2319ea0d7e69059f01e9e79d045b0243e))
* **plan:** add p5 alert priority and snooze implementation plan ([bf0a637](https://github.com/suho-han/one-click-tools/commit/bf0a637edadcce305c566beec08eeb1ac807050f))
* **plan:** add P8 monitor output and alert-rule enhancement plan ([6c82fcf](https://github.com/suho-han/one-click-tools/commit/6c82fcf86fbf8037644dd9548c5175531619c52d))
* **readme:** add easy usage guide for monitor and alert workflows ([a6e90e6](https://github.com/suho-han/one-click-tools/commit/a6e90e6feef42ffa30a0ec52536871cee70a7848))
* **readme:** reorganize localized README and CONTEXT docs ([c5c03fa](https://github.com/suho-han/one-click-tools/commit/c5c03fa094e55f8edb35a8af4b370a753d6bf01b))
* **test:** recommend GOTOOLCHAIN auto for local runs ([a4a7664](https://github.com/suho-han/one-click-tools/commit/a4a7664e07ed6c29707dc16ed06a5f3ea36eb045))
* **usage:** clarify enabled_tools filtering and agent_order output ([8b3ac2b](https://github.com/suho-han/one-click-tools/commit/8b3ac2b29ed92d881f89bbccab10f8fff8592e9f))
* **usage:** document Cursor usage sources and env vars ([0be90f0](https://github.com/suho-han/one-click-tools/commit/0be90f01763e61693da6d49e6f93a9f74e8f782e))

## 0.4.2 (2026-05-09)

### Bug Fixes
- recover from npm install conflicts during `oct update`

### Documentation
- reorganize localized README and `CONTEXT/` docs

## 0.4.0 (2026-05-03)

### Features
- Synchronized versioning with npm repository (re-basing from v0.3.7).
- Baseline update for production release.

## 0.3.7 (2026-04-30)

### Chore
- release: 0.3.7

## 0.3.6 (2026-04-28)

### Chore
- release: 0.3.6
- revert npm package name to one-click-tools

## 0.3.5 (2026-04-25)

### Features
- align agent-update icon layout with config
- improve interactive selection UX and summary output
- integrate @lobehub/icons metadata

## 0.3.4 (2026-04-20)

### Features
- add icons to agent-update and refine braille alignment
- implement high-density Braille icon renderer
- implement terminal image rendering for @lobehub/icons

## 0.3.1 (2026-04-10)

### Features
- complete v0.3.0 TODOs (Usage APIs, Package Manager detection, CI/CD, Tests)
- migrate to Go architecture (v0.3.0)

## 0.2.5 (2026-04-01)

### Features
- add config and schedule commands
- improve update UI and summary

## 0.2.2 (2026-03-25)

### Fixes
- fallback dotenv lookup to ~/.env for global oct
- resolve symlink for lib path when installed via npm global

## 0.2.0 (2026-03-15)

### Features
- copilot usage api rewrite
- support copilot billing usage query filters

## 0.1.1 (2026-03-05)

### Features
- initial release improvements
- add self-update command with beta channel

## 0.1.0 (2026-05-02)

### Features
- Initial reset release of one-click-tools (oct).
- Support for Claude, Gemini, Codex, and Copilot usage reporting.
- Automated update scheduling for macOS, Linux, and Windows.
- Interactive configuration system.
- Standardized authentication flow for all AI providers.
