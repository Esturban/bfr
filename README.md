<img src="assets/logo.svg" width="48" height="48" alt="bfr logo">

# bfr

A Go command-line client for [Buffer](https://buffer.com)'s GraphQL API
(`api.buffer.com`). List channels, create ideas and drafts, queue posts and
threads, attach images, and read or delete existing posts -- all from the
terminal. The GraphQL client lives in its own package, `bufferclient`, which
carries no project-specific defaults, so it can be imported by other Go
tools without pulling this CLI along.

## Install

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
(default: `.bfr-channels.json` next to the repo root).

## Commands

| Command | Effect |
|---|---|
| `bfr channels` | List channel ids/names, cache to `.bfr-channels.json` |
| `bfr idea <file.md>` | Post to the ideas board -- no channel attached, never posts |
| `bfr draft <channel> <file.md>` | Draft on the channel (`saveToDraft`) -- never posts |
| `bfr post <channel> <file.md>` | **Publishes live** -- queues to the channel, will post |
| `bfr thread <channel> <file.md>` | **Publishes live** -- `---`-delimited blocks become a thread |
| `bfr image <channel> <file.md> <path>` | **Publishes live** -- Drive upload, attach, queue |
| `bfr draft-image <channel> <file.md> <path>` | Draft on the channel with an image -- never posts |
| `bfr show <post-id>` | Read a post/draft back -- status, channel, text, assets |
| `bfr list [channel]` | List drafts -- id, status, channel, text, image flag (read-only) |
| `bfr delete <post-id>` | **Permanently deletes** a post/draft -- irreversible |
| `bfr version` | Print version, commit and build date |

`<channel>` is a channel id, or a channel name from `bfr channels`
(case-insensitive, resolved against the cache -- run `bfr channels` first).

Both connected channels have an unpaused queue, so `post`/`thread`/`image`
publish for real; there is no "safe" queue state for them. `idea` and
`draft`/`draft-image` are the only verbs that never reach an audience.

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

MIT -- see [LICENSE](LICENSE).
