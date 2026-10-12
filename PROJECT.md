# PROJECT — veil (inline markdown kanban for micro)

The single context hub for this project. Every session, in any app (Claude chat,
Claude Code, a terminal agent), reads this first and updates "Status" and "Handoff log"
before ending. Keep it short; details live in the files it points to.

## What we're building
A fork of the micro editor (Go) with two generic rendering features in core,
**conceal ranges** and **virtual lines**, plus a Lua **kanban plugin** that uses them to
draw Obsidian-Kanban-compatible boards inline. Core stays generic (no kanban logic in
Go); off by default (`conceal` option); diff kept clean enough to upstream.

## Owner
Marcus: EE student, comfortable with code, new to micro internals. Explain the "why"
behind design decisions. Small, reviewable commits.

## Environment
- Ubuntu on WSL2, Windows Terminal (Cascadia Mono), `LANG=en_US.UTF-8`.
- Repo at `~/src/micro` (Linux FS, never `/mnt/c`). Fork remote `origin`, upstream
  `upstream` = github.com/zyedidia/micro.
- Dev binary `~/.local/bin/micro-dev`, run with `-config-dir ~/.config/micro-dev` so
  stable `micro` is untouched.
- Baseline upstream commit: `e207f97` (2026-10-07). Go ≥ 1.19 required; use current Go.

## Source-of-truth files
| File | Holds | Who may change it |
|---|---|---|
| `PROJECT.md` | Context, status, decisions summary, handoff log | Anyone (append to log; edit Status) |
| `PLAN.md` | Phase 1 recon: architecture map, change list, risks | Recon/lead only |
| `docs/API.md` | Lua API contract | Only after an entry in COORDINATION.md "API changes" |
| `COORDINATION.md` | Task board, claims, file ownership, requests, conflicts | Any agent, own rows only |

If the claude.ai Project copy and the repo copy disagree, **the repo copy on `main` wins**.
Sync the Project copy after merging to `main`.

## Approved decisions (2026-10-12)
- D1 Decorations stored on `buffer.Line`; any edit to a line drops its conceals; plugin
  recomputes (debounced).
- D2 Reveal-on-cursor in core, per group (`"line"` or `"group"` mode).
- D3 `ConcealLines` hides whole lines (zero rows).
- D4 Columns are 0-based **rune** indices; styles are colorscheme group names.
- D5 Virtual rows map to their anchor line for clicks; cursor skips them.
- Per-cell colour in virtual lines via `StyleVirtualLine` (API v0.2).

## Key code facts (from PLAN.md)
- All screen↔buffer mapping: `internal/display/softwrap.go` (Scroll, Diff,
  SLocFromLoc, VLocFromLoc, LocFromVLoc, getRowCount).
- Drawing: `internal/display/bufwindow.go` `displayBuffer()`.
- All edits pass through `LineArray.insert/remove` (called by `SharedBuffer.insert/remove`); decorations hook in there.
- Lua sees exported `*Buffer` methods automatically (gopher-luar); no binding file.

## Status
- Phase 1 Recon: **done** (2026-10-12).
- Phase 2 Conceal: T1, T2 (PR #1) and T3 row model (PR #4) **merged**.
- Phase 3 Virtual lines: T4 drawing and T5 vertical cursor/mouse integration
  implemented on `agent-codex/vlines-movement`, build/tests/race checks pass;
  PR #5 review fixes implemented and tested; awaiting re-review. Single arrows
  enter hidden source to reveal it; virtual rows cannot hold a cursor.
- Phase 4 Kanban plugin: T6 demos merged as PR #2; T7 kanban PR #3 awaits
  T4/T5 re-review/merge and shared-doc rebase. PR #3 fixes passed; real Lua
  kanban drawing, keyboard entry, card move and undo pass against this core.
- Phase 5 Polish: not started.
- Upstream: issue/discussion not yet opened (do before Phase 2 PR).

## Open items
- [x] Baseline + T1 build/test green on WSL (2026-10-12).
- [ ] Open upstream discussion with API v0.3.
- [x] gopher-luar converts Lua tables → `[]string` (`TestDecorationsFromLua` passes).

## Handoff log (newest first; one or two lines each)
- 2026-10-12 · Codex (PR #5 fixes) · Fixed hidden-source keyboard access and unwrapped raw-column drift; regression tests, build/race suite and real kanban keyboard/drawing tests pass. Next: Claude re-review, PR #5 merge, then PR #3 rebase/merge.
- 2026-10-12 · Codex (T4/T5) · Hidden/virtual drawing and vertical movement implemented; standard tests, display/action race checks and real Lua demo runtime pass. Next: independent review/merge, then kanban fixes; installed editor unchanged.
- 2026-10-12 · chat (Claude) · T7 kanban plugin opened as PR (parser, layout, moves, toggle,
  spec tests via `go test ./plugins/...`). Codex on T3. Next: review T7; T4/T5 after T3.
- 2026-10-12 · Codex (T3) · Row model, scrolling and conceal-aware mapping implemented;
  build/tests/race checks pass. Review, then T4 drawing and T5 cursor movement.
- 2026-10-12 · chat (Claude) · Reviewed PR #1 (Codex merged it). T6 demo plugins in
  `plugins/` (not runtime/, see COORDINATION log), opened as PR. Next: T3.
- 2026-10-12 · Codex (T2 review) · Addressed Claude's three requested fixes in PR #1;
  build, standard tests and display race tests pass. Merge and T3 integration remain.
- 2026-10-12 · Codex (T2) · Conceal drawing/reveal/horizontal clicks implemented;
  standard build/tests + display race tests pass. Review, then B's T3 integration.
- 2026-10-12 · chat (Claude, agent A) · Merged T1 to main (review waived). T2/T3 open.
- 2026-10-12 · chat (Claude, agent A) · T1 tests green on WSL. Repo seeded on GitHub
  (Reid-Engineered/veil-micro). Next: review T1, merge, start T2/T3.
- 2026-10-12 · chat (Claude, agent A) · T1 written: decorations storage, API v0.3, 15 unit
  tests incl. a Lua call test. Not compiled here (Go modules blocked). Next: test on WSL.
- 2026-10-12 · chat (recon) · Phase 1 delivered: PLAN.md, COORDINATION.md, API.md v0.2,
  PROJECT.md. Marcus approved rune columns, ConcealLines, per-cell colour.
