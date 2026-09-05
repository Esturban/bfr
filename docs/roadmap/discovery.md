# API surface discovery

What Buffer's GraphQL API exposes beyond what `bfr` calls today, and which
gaps are worth closing before or after the open-source release. Ground
truth for both sides was pulled fresh, not carried over from prior tickets:

- **bfr's current calls**: read from `bufferclient/client.go` and
  `commands.go` directly (2026-09). Note for anyone relying on an older
  ticket's description of this client: `SchedulingType` is **not**
  hardcoded to `"automatic"` everywhere. `post`/`thread`/`image`/`draft`/
  `draft-image` do send `"automatic"` (queue placement), but `schedule`
  sends `mode: "customScheduled"` with no `schedulingType` at all when
  retiming an already-scheduled post. That distinction is the
  actual fix for a real retiming defect and is load-bearing, not an oversight.
- **Buffer's full surface**: read live via GraphQL introspection
  (`__schema`, `__type` on `Query` and `Mutation`) against
  `api.buffer.com`, using the repo's own `BUFFER_API_KEY`. Introspection
  is a query, not a mutation: nothing was created, edited, scheduled, or
  deleted against the live account to produce this document.

Two verbs mentioned in earlier planning material, `draft-thread` and
`draft-document`, do **not** exist in `main.go`'s command switch as of
this writing. The real verb list is: `channels`, `idea`, `draft`,
`schedule`, `post`, `thread`, `image`, `draft-image`, `attach-image`,
`update`, `show`, `list`, `delete`, `version`.

## Capability table

| Capability | bfr supports today | Worth supporting | Why |
|---|---|---|---|
| `createPost` (queue / draft, `addToQueue` mode) | Yes | Yes (kept) | Core write path behind `post`, `thread`, `image`, `draft`, `draft-image`. |
| `editPost` (`customScheduled` retime, text/asset/metadata patch) | Yes | Yes (kept) | Core write path behind `schedule`, `update`, `attach-image`. |
| `deletePost` | Yes | Yes (kept) | Core write path behind `delete`. |
| `createIdea` | Yes | Yes (kept) | Core write path behind `idea`. |
| `account { organizations }`, `channels` | Yes | Yes (kept) | Backs `channels` and the local channel cache every other verb resolves against. |
| `post` / `posts` (read) | Yes | Yes (kept) | Backs `show` and `list`. |
| `movePostInQueue` | No | No | Shuffles a post's implicit queue position without changing its content or a concrete time. `bfr schedule` already exists specifically to replace "trust Buffer's queue ordering" with an explicit `dueAt` plus a re-read confirmation (a real incident was caused by trusting implicit queue behavior). Adding a verb that leans back on implicit ordering works against a fix already shipped. |
| `aggregatedPostMetrics` (channel/date-range rollup) | No | **Yes** | Read-only analytics rollup (impressions, reactions, comments, engagement rate on LinkedIn). Downstream tooling that logs post performance already wants `engagement_24h` figures, and a weekly review wants a performance rollup. Both are currently hand-checked in Buffer's own UI or estimated. This closes a real, already-documented gap with a pure read. |
| `post.metrics` (per-post analytics) | No | **Yes** | Same justification at single-post granularity: lets `bfr show` report real engagement instead of nothing, feeding a local performance log without a browser trip. |
| `ideas` / `ideaGroups` (read back) | No | **Yes** | `bfr idea` is the only write verb with no matching read verb: an idea created through the CLI is invisible again until someone opens Buffer's UI. Every other write verb (`draft`, `post`, `schedule`, ...) has `show`/`list` as its mirror; ideas don't. Small, symmetric, read-only. |
| `createContentItem` / `*ContentItemDraft` / `promoteContentItemDraftToPosts` / `updateContentItem` / `deleteContentItem` | No | No (for now) | Buffer's own schema marks this whole family "early preview, can change without a deprecation period." It's a real capability (one item, many channel-specific variants, in one call) but building a CLI verb on an API Buffer itself won't commit to yet is the wrong trade for a small, stable tool. Revisit once Buffer marks it stable. Don't build against a moving target now. |
| `createPostTemplate` / `updatePostTemplate` / `deletePostTemplate` / `postTemplate(s)` | No | No | Buffer's template library duplicates what this whole repo already is: a markdown file is the operator's template. Adding a second, Buffer-hosted template store is two sources of truth for the same job. |
| `dailyPostingLimits` | No | No | `createPost`/`editPost` already return `LimitReachedError` inline when a limit is actually hit. A separate pre-check call adds a network round-trip to learn something the write path already tells you reactively. |
| `channel` (single-channel read) | No | No | `channels` plus the local cache already answers this at negligible cost; a second verb for one row out of an already-cached list is surface area without a real use case. |
| `post.tags` | No | No | No mutation exists to create or assign tags via this API at all: tags are managed in Buffer's own UI. Reading a field the CLI can't write is a half-feature with no operator workflow behind it. |
| `post.notes` | No | No | Notes are a team-collaboration feature (comments on a post between teammates). This is a solo-operator tool; there is no second person to leave a note for. |
| `post.allowedActions` | No | No | Interesting for hardening the existing hand-written status gates (`update`/`schedule` already refuse unsafe states explicitly), but the current explicit checks were deliberately added post-incident and are easy to audit in Go. Swapping them for a server-provided capability list trades a reviewable local check for an opaque one, for no user-visible gain today. |

## Recommended for Phase 2 (GitHub issues)

1. **`bfr metrics <post-id>`** (or fold into `show --full`): read
   `post.metrics` for a single post. *Outcome:* real LinkedIn engagement
   numbers (impressions, reactions, comments, engagement rate) land in
   a local performance log from a CLI call instead of a manual
   look at LinkedIn. *Why it made the cut:* directly fills the
   `engagement_24h` field that downstream performance-logging tooling
   already expects and currently can't populate automatically.
2. **`bfr metrics-summary <channel> [--since DATE]`**: read
   `aggregatedPostMetrics` for a channel/date-range rollup. *Outcome:*
   one command answers "how did this month's LinkedIn posts do" for the
   weekly review, instead of tallying the performance log by hand.
   *Why it made the cut:* read-only, zero risk to the honest-verb line,
   and replaces a recurring manual rollup that already happens today.
3. **`bfr ideas`**: read `ideas`/`ideaGroups` back. *Outcome:* an idea
   created via `bfr idea` becomes visible again from the CLI instead of
   only in Buffer's UI. *Why it made the cut:* every other write verb has
   a read mirror (`show`/`list`); `idea` is the one write-only verb, and
   this closes that asymmetry with a read-only addition.

Everything else in the table above is a deliberate **no**, not an
oversight: most of it either contradicts a fix already shipped
(`movePostInQueue` vs. the scheduled-retime fix), duplicates a source of truth this repo
already owns (templates vs. markdown files), or has no operator behind it
in a one-person tool (tags, notes). Keeping the verb surface small and
honest is the point; a table that recommended all of it would be a
different kind of failure than the one this discovery was asked to avoid.
