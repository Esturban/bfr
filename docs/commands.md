# Commands

`<channel>` is a channel id, or a channel name from `bfr channels`
(case-insensitive, resolved against the cache, run `bfr channels` first).

Two verbs publish live the moment they run (both connected channels have an
unpaused queue): `post`, `thread`, `image`. Everything else is safe: it
either never reaches an audience, or it edits/reads something that already
exists without changing its reach.

## channels

Lists every connected channel and caches it for name lookups elsewhere.

```sh
bfr channels
```

```
Cached 2 channel(s) to .bfr-channels.json
64f1a2b3c4d5e6f7a8b9c0d1	valest	linkedin	queuePaused=false
64f1a2b3c4d5e6f7a8b9c0d2	general-x	twitter	queuePaused=false
```

## idea

Posts to the ideas board. No channel attached, never posts.

```sh
bfr idea ./post.md
```

```
IDEA CREATED (draft only, not posted): id=64f1a2b3c4d5e6f7a8b9c0d3
```

## draft

Drafts on a channel (`saveToDraft`). Never posts.

```sh
bfr draft valest ./post.md
```

```
DRAFT on valest (not queued, not published): post id=64f1a2b3c4d5e6f7a8b9c0d4 status=draft
```

## schedule

Moves an existing draft to a scheduled time, or retimes an already
scheduled post. Never creates a new post. Optionally carries a LinkedIn
first comment, which survives a retime even when `--first-comment` is
omitted on that call (the existing one is read back and echoed rather than
silently dropped).

```sh
bfr schedule 64f1a2b3c4d5e6f7a8b9c0d4 2026-09-10T14:00:00Z --first-comment "Link in the comments."
```

```
SCHEDULED and CONFIRMED by re-read: post id=64f1a2b3c4d5e6f7a8b9c0d4 now due 2026-09-10 14:00 UTC / 2026-09-10 17:00 +03 / 2026-09-10 10:00 EDT
```

A bare success response from Buffer's API is not trusted here: `schedule`
always re-reads the post afterward and compares the *actual* resulting
`dueAt` against what was requested, failing loudly if they do not match
rather than reporting success on a silent no-op. See the source comment on
`cmdSchedule` for the two verified field combinations (draft to scheduled
vs. retiming an already-scheduled post) if you are extending this verb.

## post

**Publishes live.** Queues to the channel; will post.

```sh
bfr post valest ./post.md
```

```
QUEUED: post id=64f1a2b3c4d5e6f7a8b9c0d5 status=queued
```

## thread

**Publishes live.** `---`-delimited blocks in the file become a thread.

```markdown
First tweet in the thread.

---

Second tweet, posted as a reply.
```

```sh
bfr thread general-x ./thread.md
```

```
QUEUED: post id=64f1a2b3c4d5e6f7a8b9c0d6 status=queued
```

## image

**Publishes live.** Drive upload, attach, queue, in one call.

```sh
bfr image valest ./post.md ./cover.png
```

```
Image asset URL: https://drive.usercontent.google.com/download?id=...
QUEUED: post id=64f1a2b3c4d5e6f7a8b9c0d7 status=queued
```

## draft-image

Drafts on the channel with an image attached. Never posts.

```sh
bfr draft-image valest ./post.md ./cover.png
```

```
Image asset URL: https://drive.usercontent.google.com/download?id=...
DRAFT on valest (not queued, not published): post id=64f1a2b3c4d5e6f7a8b9c0d8 status=draft
```

## attach-image

Attaches an image to an *existing* draft or post. Never creates a new
post, never changes text, status, or schedule. Takes an already-public URL,
not a local file, so it does no Drive upload of its own and does not need
`BUFFER_DRIVE_ACCOUNT`/`gog`/`sips`.

```sh
bfr attach-image 64f1a2b3c4d5e6f7a8b9c0d4 https://raw.githubusercontent.com/Esturban/ev-assets/main/cover.png
```

```
IMAGE ATTACHED: post id=64f1a2b3c4d5e6f7a8b9c0d4 status=draft
```

## update

Replaces an existing draft's text in place. Refuses anything not
`status=draft` (read back first), never touches status, schedule, channel,
or assets.

```sh
bfr update 64f1a2b3c4d5e6f7a8b9c0d4 ./revised-post.md
```

```
UPDATED: post id=64f1a2b3c4d5e6f7a8b9c0d4 status=draft
```

## show

Reads a post or draft back: status, channel, text, assets. Text truncates
to 100 characters by default; `--full` prints it untruncated, which matters
when verifying anything appended past that point, such as hashtags.

```sh
bfr show 64f1a2b3c4d5e6f7a8b9c0d4 --full
```

```
id:      64f1a2b3c4d5e6f7a8b9c0d4
status:  draft
dueAt:   2026-09-10T14:00:00Z
channel: valest (linkedin)
text:    The full, untruncated post body appears here, hashtags and all.
firstComment: Link in the comments.
asset:   mimeType=image/png source=https://drive.usercontent.google.com/download?id=...
```

## list

Lists drafts and scheduled posts on a channel (or every channel if none is
given): id, status, channel, due time in UTC/Riyadh/New York, text, and
whether an image is attached. Read-only.

```sh
bfr list valest
```

```
id	status	channel	due(UTC)	due(Riyadh)	due(NewYork)	text	image
64f1a2b3c4d5e6f7a8b9c0d4	draft	valest			The full, untruncated post b...	yes
64f1a2b3c4d5e6f7a8b9c0d5	scheduled	valest	2026-09-10 14:00 UTC	2026-09-10 17:00 +03	2026-09-10 10:00 EDT	Another draft's text...	no
```

## delete

**Permanently deletes** a post or draft. Irreversible.

```sh
bfr delete 64f1a2b3c4d5e6f7a8b9c0d4
```

```
DELETED: post id=64f1a2b3c4d5e6f7a8b9c0d4
```

## version

```sh
bfr version
```

```
bfr v0.3.0 (commit 2142ded, built 2026-09-04T14:00:44Z)
```
