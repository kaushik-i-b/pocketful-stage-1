# Factory

This file records the setup that ran the submitted room, the times that were measured, and the checks that were saved. Token counts and dollar cost for this run were not measured.

## Seats

Band Desktop 0.4.12 created three owned coding-agent seats. Each seat is a Cursor ACP runtime (`agent acp`, Cursor CLI `2026.10.01-e373342`) on this machine. Band rejected an explicit runtime model of `auto` and of `default[]`, so the seats were created without a model flag and used the signed-in Cursor default. The CLI lists that default as `auto` (display name Auto). The mandates record that value. They do not name a more specific model, because the runtime did not.

| Seat | Handle | Id | Worktree branch |
| --- | --- | --- | --- |
| Coordinator | `kaushikitagib/coordinator` | `bbf45910-8711-42c8-bf14-6a320fc6343a` | `seat/coordinator` |
| Developer | `kaushikitagib/developer` | `53084c22-3bc6-4d04-8295-d65aa22ea125` | `seat/developer` |
| Reviewer | `kaushikitagib/reviewer` | `62fb0b31-8b2b-4bd9-9e9e-a8fe1ac1af29` | `seat/reviewer` |

The three worktrees share one Git object store. Each seat commits only in its own worktree. ACP does not take Band owner instructions, so each worktree also has a git-excluded Cursor rule with the same text as `mandates/`. Those rules are not part of this repository.

Docker Sandbox was not used. The reviewer ran the official harness on the host, against a checkout of the commit named in the room.

## Room

Submission room: `09670909-0fca-4b28-abe1-a6df7191f4ee` (title "Pocketful stage 1").

The only human task message is `25f83974-72bd-44c5-9757-f06d7dee2bb7`, sent at `2026-10-06T05:03:25.715327Z` (10:33:25 IST). It mentions only the Coordinator. No later human message was sent.

`room.json` is not in this repository. The Band CLI can list messages. It cannot download the official full session. That file still has to be saved from the Band console: open this room, use the room menu, choose Download, then Download full session. Download filtered is the wrong file.

## What happened

Times below are from the room messages, the Git commits, and the harness reports.

1. `2026-10-06T05:03:25Z` — human dispatch.
2. `2026-10-06T05:05:33Z` — Coordinator handed the task to the Developer.
3. `2026-10-06T05:08:36Z` — a saved isolated report `dev-stage1-impl1` records 147 passed with provenance `working-tree` and revision `5e7d08797aec4b15cc0027326ddb526cc4fff662`. That revision is the empty repository commit. The report scored the worktree, not that commit's tree.
4. `2026-10-06T05:11:19Z` (10:41:19 IST) — Developer commit `9efc373661c0135a72f75e53d513b4d007be9040`, "Implement the stage 1 payments and settlements service."
5. `2026-10-06T05:12:00Z` and `2026-10-06T05:15:24Z` — isolated reports `review-stage1-9efc373` and `review-stage1-9efc373-r2` record stage 1 pass, 147 passed, 0 failed, for `9efc373661c0135a72f75e53d513b4d007be9040`.
6. `2026-10-06T05:15:44Z` (10:45:44 IST) — Developer commit `7e0beb83b35c3091bf054c7607c5f1c7b7613a7a`, "Keep minor-unit amounts exact and unknown handles as not found." It changes `stage-1/fixture.go`, `handlers.go`, `jsonutil.go`, `main.go`, and `state.go`. The message list read from the CLI did not contain a rejection of `9efc373`. Both saved checks of that earlier commit record a shipped stage 1 pass.
7. `2026-10-06T05:16:35Z` — Developer handed `7e0beb83b35c3091bf054c7607c5f1c7b7613a7a` to the Reviewer and the Coordinator.
8. `2026-10-06T05:17:28Z` to `2026-10-06T05:17:46Z` — Reviewer isolated run `review-7e0beb8-20261006104728`. `report.json` says revision `7e0beb83b35c3091bf054c7607c5f1c7b7613a7a`, mode `isolated`, stage 1 pass, 147 passed, 0 failed, 0 errors, `claimed_stage` 1, `highest_contiguous` 1, `overshoot` null. `stage-2.log` in that directory records one failure, `test_routes_are_directly_navigable[/-pay-submit]`, a Playwright timeout waiting for `[data-testid='login-email']`, then the suite stopped.
9. `2026-10-06T05:18:48Z` — Reviewer reported that result in the room.
10. `2026-10-06T05:19:11.723331Z` (10:49:11 IST) — Coordinator: "Stage 1 is verified. No repair round is open." Same revision and the same report path.

From the human dispatch to that Coordinator report is 15 minutes 46 seconds.

After the report, the three seats continued to post that no reply was needed. At `2026-10-06T05:30:59Z` that loop was still the newest text in the room. No commit was added after `7e0beb83b35c3091bf054c7607c5f1c7b7613a7a`. Those messages were left alone. A full-session download taken while the loop continues will include them.

## Independent check

After the Coordinator report, the same official command was run again on a separate checkout of `7e0beb83b35c3091bf054c7607c5f1c7b7613a7a`. The service files were not edited.

Output directory: `band-factory/state/checks/results/packaging-isolated-7e0beb8` (also written first to `/tmp/pocketful-isolated-7e0beb8`).

- Started `2026-10-06T05:19:55.245867Z`, finished `2026-10-06T05:20:15.056078Z`
- Process exit 0
- Stage 1 pass: 147 passed, 0 failed, 0 errors
- Claimed stage 1
- The command also printed `stage 2: fail`
- `report.json` field `overshoot` is null

## Costs

`band usage rooms` and `band usage agents` on this machine did not attribute any tokens to this room. The only row they printed was unattributed usage whose last activity is `2026-08-14T16:47:08.272Z`. That row is not this run. No dollar figure and no token count are claimed for the submitted room.

## What the check caught

The saved stage 1 logs for both Developer commits report 147 passed. The follow-up commit is still in history; it was not produced by a saved stage 1 failure. The stage 2 log failed one UI navigation test and stopped. That failure is why this factory claims stage 1 only.
