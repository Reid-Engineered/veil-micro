PROJECT: veil — inline markdown kanban for the micro editor (Go fork + Lua plugin)

You are one of several AI sessions (Claude chat, Claude Code, other agents) working on this project, possibly at the same time. Shared state lives in files, not in any one conversation.

START OF EVERY SESSION
1. Read PROJECT.md first (context, approved decisions, status, handoff log).
2. If you will touch the repo, read COORDINATION.md and claim a task (agent name, task, files, ISO timestamp) before editing anything. Never start a task or edit a file another agent has claimed; write a request in COORDINATION.md instead.
3. Code against docs/API.md. Never change the API without first adding an entry under "API changes" in COORDINATION.md.

END OF EVERY SESSION
- Update PROJECT.md: the Status section and one or two lines at the top of the Handoff log (date · app/agent · what changed · what's next).
- Mark your COORDINATION.md tasks done or released.
- If you can't write to the repo (e.g. a chat session), give Marcus the exact text to paste into PROJECT.md / COORDINATION.md.

SOURCE OF TRUTH
- The repo copy on main wins over the claude.ai Project copy. If they disagree, say so and use the repo copy.
- PLAN.md = architecture map and risks. docs/API.md = Lua API contract. COORDINATION.md = task board and file ownership.

HARD RULES
- Core Go changes stay generic: no kanban logic in Go. All kanban behavior lives in the Lua plugin.
- Feature is off by default (buffer option `conceal`); zero cost when unused.
- Keep the diff minimal and upstreamable. Flag anything that would make upstreaming unlikely.
- Never commit to main. Work in your own git worktree on branch agent-<name>/<task>; rebase on main before a small, reviewed merge.
- On conflict or overlap: stop and record it in COORDINATION.md; never overwrite another agent's work.
- Columns are 0-based rune indices; styles are colorscheme group names.

ENVIRONMENT
- Ubuntu on WSL2, Windows Terminal (Cascadia Mono), LANG=en_US.UTF-8.
- Repo at ~/src/micro on the Linux filesystem (never /mnt/c).
- Dev build installed as ~/.local/bin/micro-dev, run with -config-dir ~/.config/micro-dev; the stable micro must keep working.

WORKING STYLE
- Marcus is an EE student, comfortable with code but new to micro's internals: explain the "why" behind design decisions.
- Small, reviewable commits with clear messages, one phase at a time; stop and report at the end of each phase.
- Be direct about risks and unverified claims (say what you did not build or test).
