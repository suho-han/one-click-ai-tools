# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

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
