# Library usage

`bufferclient` is a standalone Go package with no project-specific
defaults, so it can be imported by other Go tools without pulling the CLI
along.

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

## What it never does

`bufferclient` never calls a share-immediately mutation. Every write is one
of:

- `mode: addToQueue` (queues, and publishes for real once the channel's
  queue is unpaused)
- `saveToDraft: true` (never leaves draft state)
- `createIdea` (no channel attached at all)

## Editing an existing post

`EditPost` is `CreatePost`'s sibling mutation: it changes an existing
post/draft instead of creating a new one. Only `ID` is required; every
other field is optional and omitted when unset, so a caller that only
wants to attach an asset sends nothing else, no text, no mode, no
scheduling fields.

```go
result, _, err := c.EditPost(bufferclient.EditPostInput{
    ID:     "existing-post-id",
    Assets: []map[string]interface{}{{"image": map[string]string{"url": "https://..."}}},
})
```

The same struct carries `Text`, `Mode`, `SchedulingType`, `DueAt`,
`SaveToDraft`, and `Metadata`, which is what lets `bfr schedule` move a
draft to a scheduled time, retime an already-scheduled post, and set a
LinkedIn first comment, all through the one mutation.
