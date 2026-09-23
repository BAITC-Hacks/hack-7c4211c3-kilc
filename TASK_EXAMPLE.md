# TASK_EXAMPLE.md

How to write a task prompt for a coding agent (Codex) on this project.
`AGENTS.md` holds the rules that apply to every task. A task prompt holds
only what is specific to this one task: which files, what result, how to
check it.

## Workflow

1. Pick the next item from your area of the roadmap.
2. Paste the **task-writer prompt** below into a chat agent, with a short
   description of the feature.
3. Read the generated task. Answer every question under
   "Decisions already made" yourself — do not let the agent guess them.
4. Run the checklist at the bottom.
5. Give the task to Codex in a fresh session on your own branch.
6. Check the report, run the verify command yourself, commit using the
   commit message from the report (edit the `Час` line if needed).

One task = one commit = 10–15 minutes of agent work. If the file list
does not fit on one line, split the task.

## Task-writer prompt

Copy everything inside the block, fill in the last two lines.

```text
You write task prompts for a coding agent. The agent works in a Go repo
whose standing rules are in AGENTS.md (attached / quoted below). Do not
repeat those rules in the task. Output only the task, in exactly this
format:

## Task: <one line, imperative>

**Files:** <comma-separated list; the agent may create or edit only these>

**Goal:** <2–3 sentences: what exists after this task that did not before>

**Contract:**
- Input: <types, request shape, form fields>
- Output: <types, response shape, status codes, page content>
- Depends on: <existing functions, tables, routes it may use, by name>

**Decisions already made:**
- <every choice the agent would otherwise guess; if unknown, write
  "QUESTION: ..." so the human answers it before sending>

**Acceptance:**
- <verify command from AGENTS.md>
- <concrete case: input X -> output Y>
- <edge case: input Z -> output W>

**Not in this task:** <adjacent work the agent must leave alone>

**Report back:** changed files, verify output, assumptions made, and a
commit message in COMMITS.md format (type(scope): описание, Час, AI,
Проверка).

Constraints for you:
- Keep the task to one commit, roughly 10–15 minutes of agent work.
  If it is bigger, output several tasks in order.
- Acceptance cases must be concrete values, never "works correctly".
- Never invent product rules. Anything not in AGENTS.md or my
  description becomes a QUESTION line.

My area: <constructor+AI | rating+catalog | platform+proposals>
Feature: <what you want built, 1–5 sentences>
```

## Example output

This is what a good task looks like. The thresholds under "Decisions
already made" are placeholders showing the level of detail needed; the
team agrees the real values before sending.

```markdown
## Task: Implement the task readiness rating

**Files:** rating/rating.go, rating/rating_test.go

**Goal:** A pure function that scores a task card 0–100 using the 7
weighted components from AGENTS.md, counting only confirmed fields, and
returns a per-component breakdown plus a list of what would raise the
score.

**Contract:**
- Input: `rating.Card` — one `Field{Text string, Confirmed bool}` per
  card field (context, need, users, data, constraints, expected_result,
  success_criteria, contact, interaction_format). Defined in this file;
  no imports from store/, api/, ai/.
- Output: `Result{Total, Potential int; Level string;
  Components []Component; Missing []string}`,
  `Component{Name string; Max, Score int}`.
  `Potential` is the score if every filled field were confirmed.
- Depends on: nothing.

**Decisions already made:**
- A field is filled if its trimmed text is ≥ 20 characters; 1–19
  characters gives half the points; empty gives 0.
- Unconfirmed fields score 0 in `Total` but count in `Potential`.
- context_and_need = context 10 + need 10.
- business_contact = contact 5 + interaction_format 5.
- Level names: draft, working, ready, priority.
- `Missing` entries are Russian, one per component below its max, e.g.
  "Опишите доступные данные или примеры (+20)".

**Acceptance:**
- `go test ./rating/...` passes.
- Empty card -> Total 0, Potential 0, Level "draft", 7 Missing entries.
- All fields filled and confirmed -> Total 100, Level "priority",
  Missing empty.
- All fields filled, none confirmed -> Total 0, Potential 100.
- Totals 39/40, 69/70, 89/90 map to draft/working, working/ready,
  ready/priority.
- `go list -deps ./rating` shows no store, api or ai package.

**Not in this task:** HTTP endpoint, live recalculation JS, storing the
score in the database, catalog sorting.

**Report back:** changed files, test output, assumptions, commit message.
```

## Checklist before sending to Codex

- [ ] Files are listed, and none of them belong to another teammate's area
      (`main.go` and `schema.sql` belong to the platform owner — ask them).
- [ ] No `QUESTION:` lines left.
- [ ] Every acceptance line has concrete values.
- [ ] The contract matches what the other side actually calls or returns.
- [ ] "Not in this task" names the obvious next step, so the agent stops.
- [ ] The task does not break a graded rule: manual team selection,
      low-rated tasks stay visible, AI adds no facts, human confirms
      before publishing.
