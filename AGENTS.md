# AGENTS.md

## Project

MVP for a hackathon case: a business rep turns a rough task description
into a structured card, the system scores its completeness 0-100 and
publishes it to an open catalog. Student teams submit proposals; the
business manually accepts or rejects them.

Hard deadline: 18:00. Working code beats complete code.

## Stack

- Language/runtime: Go 1.25+ (required by the SQLite driver; older Go auto-downloads the toolchain)
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
- `schema.sql`; seed fixtures are `data/*.json`
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
- Only filled and confirmed fields score. Editing a field removes it
  from `confirmed` until the user confirms it again.
- Levels: 0-39 draft, 40-69 working, 70-89 ready, 90-100 priority.
  Level is derived from score in Go, never stored.
- A low rating never hides a task from the catalog and never blocks a
  proposal. All published tasks are visible to every team.
- Catalog sorts by rating descending, with filters by category, industry
  and level (level filter is a score range).
- Category is one code from the fixed list below. The AI may suggest a
  category only from that list or leave it empty; the user confirms it.
  Category does not affect the rating.
- Team selection is manual only. No code path may set a proposal to
  accepted without an explicit user action. Never auto-assign.
  The business may accept one, several or none.
- Stage confirmation is a manual business action on an accepted
  proposal only. It sets `stage_confirmed_at` and `points_awarded`.
  Team points are `SUM(points_awarded)`, never stored on the team.
- No personal or sensitive attributes of team members are stored.
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
Task: id, company, title, industry, category,
      draft_text, qa(json [{field, question, answer}]),
      context, need, users, data, constraints,
      expected_result, success_criteria, contact, interaction_format,
      confirmed(json list of field names),
      status(draft|published), score, created_at, published_at
Team: id, name, interests(json list of category codes), skills, tech
Proposal: id, task_id, team_id, idea, plan, deadline(YYYY-MM-DD),
          prototype_url, status(pending|accepted|rejected),
          decided_at, stage_confirmed_at, points_awarded, created_at
```

- `draft_text` is the original rough description, kept unchanged.
- `qa` holds the clarifying questions and the user's answers.
- `score` is stored for sorting and rewritten on every save.
- `deadline` is required and must parse as a date; `prototype_url` is
  required and must start with `http://` or `https://`.
- Timestamps are `TEXT` in RFC 3339.

Categories (store the code, show the label; enforce with `CHECK`):

| Code | Label |
|---|---|
| `crm` | CRM и продажи |
| `automation` | Автоматизация процессов и администрирование |
| `analytics` | Аналитика и отчётность |
| `ai_assistant` | Чат-бот / AI-ассистент |
| `web_app` | Веб-сервис или приложение |
| `integration` | Интеграция систем и данных |
| `content` | Обучение и контент |
| `other` | Другое |

Role is a dropdown, not a user account: business or student team.

## Out of scope

Auth, registration, password reset, role model, real-time chat,
notifications, file storage, mobile layout, deployment infra,
ML training, vector DB, project tracker.
