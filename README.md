<img src="assets/logo.svg" width="48" height="48" alt="bfr logo">

# bfr

A Go command-line client for [Buffer](https://buffer.com)'s GraphQL API
(`api.buffer.com`). List channels, create ideas and drafts, queue posts and
threads, attach images, and read or delete existing posts, all from the
terminal. The GraphQL client lives in its own package, `bufferclient`, which
carries no project-specific defaults, so it can be imported by other Go
tools without pulling this CLI along.

## ⚠ Which verbs publish live: read this before wiring up an agent

`bfr` is built to be driven by an agent, not just a human at a terminal.
That means the single fact that matters most is which commands are safe to
run unattended and which ones put text or an image in front of a real
audience. Full behavior for every verb is in the [Commands](#commands)
table below. This is just the split that must never get lost in a skim.

**Never publishes (safe to script/automate):**
`channels`, `idea`, `draft`, `draft-image`, `show`, `list`, `version`

**Publishes live to a real audience:**
`post`, `thread`, `image`

**Mutates an existing post/draft in place (not a new publish, but not a no-op either):**
`schedule` (retimes), `attach-image` (adds an image), `update` (replaces text)

**Irreversible:**
`delete`

Both connected channels run an unpaused queue, so there is no "safe" queue
state for `post`/`thread`/`image`: calling them **will** post for real.

## Install

Full documentation: https://esturban.github.io/bfr/

```sh
git clone https://github.com/Esturban/bfr.git
cd bfr
go build -o bfr .
```

Or, if you already have the module available to your Go toolchain:

```sh
go install github.com/Esturban/bfr@latest
```

Requires Go 1.21+.

## Auth

`bfr` reads `BUFFER_API_KEY` from the environment, or from a `.env` file at
the repo root (gitignored, never committed). Generate a token from your
Buffer account's API access / developer app settings. Never pass it as a
flag, it is never echoed or logged.

```sh
# .env
BUFFER_API_KEY=your-token-here
```

The image verbs (`image`, `draft-image`) additionally require:

- `BUFFER_DRIVE_ACCOUNT`: a `gog`-authenticated Google Drive account
  email, used to upload the image and share it before attaching it to the
  post. No default: it must be set explicitly.
- The `gog` and `sips` CLIs on PATH (`sips` is macOS-native, used to
  convert the source image to real JPEG bytes before upload).

`BUFFER_CACHE_FILE` optionally overrides where the channel cache is written
(default: `.bfr-channels.json` next to the repo root).

## Commands

| Command | Effect |
|---|---|
| `bfr channels` | List channel ids/names, cache to `.bfr-channels.json` |
| `bfr idea <file.md>` | Post to the ideas board: no channel attached, never posts |
| `bfr draft <channel> <file.md>` | Draft on the channel (`saveToDraft`), never posts |
| `bfr schedule <post-id> <ISO8601-datetime>` | Move an EXISTING draft to a scheduled time, or retime an already-scheduled post. Never creates a new post, confirms the change by re-reading it back |
| `bfr post <channel> <file.md>` | **Publishes live**: queues to the channel, will post |
| `bfr thread <channel> <file.md>` | **Publishes live**: `---`-delimited blocks become a thread |
| `bfr image <channel> <file.md> <path>` | **Publishes live**: Drive upload, attach, queue |
| `bfr draft-image <channel> <file.md> <path>` | Draft on the channel with an image, never posts |
| `bfr attach-image <post-id> <url>` | Attach an image to an EXISTING draft/post. Never creates a new one, never changes text/status/schedule |
| `bfr update <post-id> <file.md>` | Replace an EXISTING draft's text. Refuses anything not a draft, never touches status/schedule/channel/assets |
| `bfr show <post-id> [--full]` | Read a post/draft back: status, channel, text, assets. Text truncates to 100 chars by default; `--full` prints it untruncated (needed to verify anything appended past that point, e.g. hashtags) |
| `bfr list [channel]` | List drafts AND scheduled posts: id, status, channel, due time in UTC/Riyadh/New York, text, image flag (read-only) |
| `bfr delete <post-id>` | **Permanently deletes** a post/draft, irreversible |
| `bfr version` | Print version, commit and build date |

`<channel>` is a channel id, or a channel name from `bfr channels`
(case-insensitive, resolved against the cache: run `bfr channels` first).

Both connected channels have an unpaused queue, so `post`/`thread`/`image`
publish for real; there is no "safe" queue state for them. `idea` and
`draft`/`draft-image` are the only verbs that never reach an audience.

`attach-image` takes an already-public URL (not a local file path) and
edits an existing post/draft in place: it does no Drive upload of its
own and does not require `BUFFER_DRIVE_ACCOUNT`/`gog`/`sips`. Use
`image`/`draft-image` when the image is still a local file.

## The silent-no-op defect (schedule)

Every post created through the normal queue path carries
`schedulingType: "automatic"`. Buffer derives its `dueAt` from the
channel's own posting schedule rather than storing an exact pinned time.
Sending a bare `dueAt` to `editPost` on such a post returns
`PostActionSuccess` and silently changes nothing: the mutation succeeds,
the response looks correct, and the time never moves. That happened for
real on 2026-08-30: ten posts sat scheduled at 20:00 UTC (23:00 Riyadh),
and repeated attempts to move them all reported success while doing
nothing.

The fix is `mode: "customScheduled"` alongside `dueAt`: that is the field
that actually pins an exact time; `dueAt` alone is not. `bfr schedule`
sends it on every call, and because a success response is exactly what
this defect looks like, it never trusts the mutation's own response: it
always re-reads the post afterward and compares the ACTUAL resulting
`dueAt` to what was requested (and confirms status is still `scheduled`),
failing loudly if they don't match rather than reporting success.

`bfr schedule` also uses different field combinations depending on the
post's starting status, both found by live trial against the real API:
moving a *draft* to scheduled additionally requires an explicit
`schedulingType: "automatic"` and `saveToDraft: false`, or the post
silently stays a draft despite a success response; retiming an
already-*scheduled* post needs `mode: customScheduled` alone. Adding
`schedulingType` or `saveToDraft` there is not part of the verified working
mutation.

## Example

```sh
bfr channels                          # cache channel ids/names once
bfr draft general-x ./post.md         # draft on a channel, never posts
bfr show <post-id>                    # confirm it landed as a draft
bfr list general-x                    # see all drafts on that channel
```

A thread file uses `---` on its own line to split blocks; the first
non-empty block is the post text, the rest become thread entries:

```markdown
First tweet in the thread.

---

Second tweet, posted as a reply.
```

```sh
bfr thread general-x ./thread.md      # publishes live
```

## Library usage

```go
import (
    "os"

    "github.com/Esturban/bfr/bufferclient"
)

func main() {
    c := bufferclient.New(os.Getenv("BUFFER_API_KEY"), "")

    org, _, err := c.OrganizationID()
    if err != nil {
        panic(err)
    }

    result, _, err := c.CreatePost(bufferclient.PostInput{
        Text:           "hello from bufferclient",
        ChannelID:      "channel-id-from-bfr-channels",
        SchedulingType: "automatic",
        Mode:           "addToQueue",
        SaveToDraft:    true, // omit to queue for real
    })
    if err != nil {
        panic(err)
    }
    _ = org
    _ = result
}
```

`bufferclient` never calls a share-immediately mutation: every write is
`mode: addToQueue` (queues, and publishes for real once the channel's queue
is unpaused), `saveToDraft: true` (never leaves draft state), or
`createIdea` (no channel attached at all).

## License

MIT. See [LICENSE](LICENSE).
