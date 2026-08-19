<img src="assets/logo.svg" width="48" height="48" alt="buf logo">

# buf

A Go command-line client for [Buffer](https://buffer.com)'s GraphQL API
(`api.buffer.com`). List channels, create ideas and drafts, queue posts and
threads, attach images, and read or delete existing posts -- all from the
terminal. The GraphQL client lives in its own package, `bufferclient`, which
carries no project-specific defaults, so it can be imported by other Go
tools without pulling this CLI along.

## Naming

`buf` collides with [buf.build](https://buf.build)'s protobuf CLI, which
also installs a `buf` binary and is common on developer PATHs. This repo has
not been renamed yet -- that decision needs a matching edit to `go.mod`, the
import in `commands.go`, and the release config (`.goreleaser.yaml`), none
of which this pass touches. Until it is renamed: if you have buf.build's
`buf` installed, expect a PATH collision, and disambiguate with a full path
or a shell alias.

## Install

```sh
git clone https://github.com/Esturban/buf.git
cd buf
go build -o buf .
```

Or, if you already have the module available to your Go toolchain:

```sh
go install github.com/Esturban/buf@latest
```

Requires Go 1.21+.

## Auth

`buf` reads `BUFFER_API_KEY` from the environment, or from a `.env` file at
the repo root (gitignored, never committed). Generate a token from your
Buffer account's API access / developer app settings -- never pass it as a
flag, it is never echoed or logged.

```sh
# .env
BUFFER_API_KEY=your-token-here
```

The image verbs (`image`, `draft-image`) additionally require:

- `BUFFER_DRIVE_ACCOUNT` -- a `gog`-authenticated Google Drive account
  email, used to upload the image and share it before attaching it to the
  post. No default -- it must be set explicitly.
- The `gog` and `sips` CLIs on PATH (`sips` is macOS-native, used to
  convert the source image to real JPEG bytes before upload).

`BUFFER_CACHE_FILE` optionally overrides where the channel cache is written
(default: `.buf-channels.json` next to the repo root).

## Commands

| Command | Effect |
|---|---|
| `buf channels` | List channel ids/names, cache to `.buf-channels.json` |
| `buf idea <file.md>` | Post to the ideas board -- no channel attached, never posts |
| `buf draft <channel> <file.md>` | Draft on the channel (`saveToDraft`) -- never posts |
| `buf post <channel> <file.md>` | **Publishes live** -- queues to the channel, will post |
| `buf thread <channel> <file.md>` | **Publishes live** -- `---`-delimited blocks become a thread |
| `buf image <channel> <file.md> <path>` | **Publishes live** -- Drive upload, attach, queue |
| `buf draft-image <channel> <file.md> <path>` | Draft on the channel with an image -- never posts |
| `buf show <post-id>` | Read a post/draft back -- status, channel, text, assets |
| `buf list [channel]` | List drafts -- id, status, channel, text, image flag (read-only) |
| `buf delete <post-id>` | **Permanently deletes** a post/draft -- irreversible |
| `buf version` | Print version, commit and build date |

`<channel>` is a channel id, or a channel name from `buf channels`
(case-insensitive, resolved against the cache -- run `buf channels` first).

Both connected channels have an unpaused queue, so `post`/`thread`/`image`
publish for real; there is no "safe" queue state for them. `idea` and
`draft`/`draft-image` are the only verbs that never reach an audience.

## Example

```sh
buf channels                          # cache channel ids/names once
buf draft general-x ./post.md         # draft on a channel, never posts
buf show <post-id>                    # confirm it landed as a draft
buf list general-x                    # see all drafts on that channel
```

A thread file uses `---` on its own line to split blocks; the first
non-empty block is the post text, the rest become thread entries:

```markdown
First tweet in the thread.

---

Second tweet, posted as a reply.
```

```sh
buf thread general-x ./thread.md      # publishes live
```

## Library usage

```go
import (
    "os"

    "github.com/Esturban/buf/bufferclient"
)

func main() {
    c := bufferclient.New(os.Getenv("BUFFER_API_KEY"), "")

    org, _, err := c.OrganizationID()
    if err != nil {
        panic(err)
    }

    result, _, err := c.CreatePost(bufferclient.PostInput{
        Text:           "hello from bufferclient",
        ChannelID:      "channel-id-from-buf-channels",
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

MIT -- see [LICENSE](LICENSE).
