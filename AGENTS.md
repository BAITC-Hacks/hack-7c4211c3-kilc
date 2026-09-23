# AGENTS.md

## Project

MVP for a hackathon case: a business rep turns a rough task description
into a structured card, the system scores its completeness 0-100 and
publishes it to an open catalog. Student teams submit proposals; the
business manually accepts or rejects them.

Hard deadline: 18:00. Working code beats complete code.

## Stack

- Language/runtime: Go 1.22+
- HTTP: `net/http` stdlib. No Gin, Echo, Chi, or any web framework.
- Templates: `html/template` stdlib, server-rendered.
- Storage: SQLite via `modernc.org/sqlite` (pure Go, no CGO).
  File `data.db`. Schema in `schema.sql`, applied on startup if absent.
  Never switch to `mattn/go-sqlite3` — it needs CGO and will not build here.
- Frontend: server-rendered templates plus vanilla JS for live rating
  recalculation and proposal submission. No Node, no npm, no build step.
- Assets: all templates and static files embedded via `embed.FS`.
- Run: `go run .` — works with no API key, `STUB=1` by default.
- Verify: `go test ./rating/...` then `go run .` and check
  `curl -f localhost:8080/health`.

Do not add dependencies without being asked. The only third-party module
is the SQLite driver. Do not introduce an ORM, a web framework, a
migration tool, or a JS toolchain.

## Layout

- `rating/` — scoring function. Pure Go, no I/O, no database, no HTTP.
- `ai/` — the two model calls, both behind one interface with a stub.
- `store/` — SQLite access.
- `api/` — HTTP handlers.
- `templates/`, `static/` — UI, embedded.
- `data/` — seed fixtures.
- `schema.sql`, `seed.sql`
- `PROGRESS.md`, `COMMITS.md` — do not edit unless asked.

## Rules

1. Touch only the files named in the task. Never refactor adjacent code.
2. After every change, run the verify command and report the result.
   If it fails, fix it before reporting done.
3. No placeholder code, no `TODO`, no commented-out blocks left behind.
4. Errors surface to the user as a clear message. Never swallow an error.
   Handlers return an explicit status and a readable body.
5. If a task is ambiguous, pick the simplest reading and state the
   assumption. Do not ask and wait.
6. Never rewrite git history. No rebase, squash, amend, force push.
7. `rating/` must stay free of imports from `store/`, `api/`, or `ai/`.

## Domain rules (these are graded, do not deviate)

- Rating is 7 components summing to 100: context_and_need 20,
  data_and_materials 20, expected_result 15, success_criteria 15,
  constraints 10, users 10, business_contact 10.
- Rating returns a breakdown per component plus a list of what is
  missing, never a bare number. Recalculated after every edit.
- Levels: 0-39 draft, 40-69 working, 70-89 ready, 90-100 priority.
- A low rating never hides a task from the catalog and never blocks a
  proposal. All published tasks are visible to every team.
- Catalog sorts by rating descending, with filters by industry and level.
- Team selection is manual only. No code path may set a proposal to
  accepted without an explicit user action. Never auto-assign.
- The AI must not introduce facts the user did not supply. Validate that
  model output contains only keys from the schema; drop anything else.
- Every AI-generated field is human-confirmed before publication.
- Seed data on first run if the database is empty: 5 drafts, 5 cards,
  5 team profiles, 5 proposals.

## AI calls

Exactly two, both behind one Go interface with a stub implementation
selected by `STUB=1`:

1. draft text -> at least 3 clarifying questions, as `[{field, question}]`
2. draft + answers -> task card, as a JSON object with the fixed field set

Both paths must work with no API key present. On malformed model output,
fall back to the stub and log it. Never fabricate a successful result.

## Data model

```
Task: id, title, context, need, users, data, constraints,
      expected_result, success_criteria, contact, interaction_format,
      industry, status(draft|published), score, level, created_at
Team: id, name, interests, skills, tech
Proposal: id, task_id, team_id, idea, plan, deadline, prototype_url,
          status(pending|accepted|rejected), created_at
```

Role is a dropdown, not a user account: business or student team.

## Out of scope

Auth, registration, password reset, role model, real-time chat,
notifications, file storage, mobile layout, deployment infra,
ML training, vector DB, project tracker.
