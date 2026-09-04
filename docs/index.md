# bfr

A Go command-line client for [Buffer](https://buffer.com)'s GraphQL API
(`api.buffer.com`). List channels, create ideas and drafts, queue posts and
threads, attach images, and read or delete existing posts, all from the
terminal.

The GraphQL client lives in its own package, `bufferclient`, which carries
no project-specific defaults, so it can be imported by other Go tools
without pulling this CLI along. See [Library usage](library.md).

## Where to start

- [Install](install.md) the binary or build from source
- [Auth](auth.md) it against your Buffer account
- Read [Commands](commands.md) for every verb, one real invocation each,
  and what it prints back

## Safety model, at a glance

Two verb families:

- **Never reaches an audience**: `idea`, `draft`, `draft-image`. Nothing
  they touch is queued or published.
- **Publishes live**: `post`, `thread`, `image`. Both connected channels
  run an unpaused queue, so these post for real, immediately.

Everything else (`attach-image`, `update`, `schedule`, `show`, `list`,
`delete`) edits or reads an *existing* post/draft rather than creating a
new one, and each is scoped to touch only what its name says: `update`
never touches an image, `attach-image` never touches text, `schedule`
never touches the channel.
