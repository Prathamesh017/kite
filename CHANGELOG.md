# Changelog

All notable changes to Kite are listed here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). Each finished phase is a minor version while Kite is below 1.0.

## [Unreleased]

## [0.1.0] - 2026-10-14

Phase 1: from LLM to coding agent.

### Added

- Terminal UI with Bubble Tea: header, activity, tasks, terminal, and chat panels, with themes (ctrl+t) and mouse scrolling.
- Model client for any OpenAI-compatible chat endpoint, set in `.env` (`API_KEY`, `TEXT_MODEL_NAME`, `API_URL`).
- Agent loop: the model can call tools, get the results back, and continue until it answers.
- Tools: `list_files`, `read_file`, and `write_file`.
- Stop rules: a maximum number of iterations (`MAX_ITERATIONS`) and tool calls (`MAX_TOOL_CALLS`). A reply cut off by the length limit is marked.
- Retries with backoff for failed model requests (`MAX_RETRIES`, `RETRY_INITIAL_DELAY`, `RETRY_MAX_DELAY`).
- A timeout per request (`REQUEST_TIMEOUT`, default 60s) and per run (1 minute). On a run timeout, the question goes back into the input.
- Cancel with Esc. ctrl+c still quits.
- Task mode: the model splits a request into tasks, each run on its own with the results of the earlier ones. A short summary is shown when all tasks pass.
- Verification policies for tasks: `none`, `tool_result`, and `read_back`. A `read_back` task is checked in a fresh request that reads the files back and replies PASS or FAIL.
- Tasks list the files they will create, and no task runs if any of those files already exists.
- Tests for cancel, timeout, tool runs, task checks, and blocked files (`task test`).
- Debugging with Delve (`task debug`).

### Security

- `read_file` refuses `.env`, `.env.*`, `*.pem`, and anything under `.git/`, including through symlinks.
- `write_file` only creates new files inside the workspace and never overwrites.

[Unreleased]: https://github.com/Prathamesh017/KITE/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Prathamesh017/KITE/releases/tag/v0.1.0
