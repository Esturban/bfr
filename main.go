// bfr is a command-line client for Buffer's GraphQL API
// (https://api.buffer.com). No verb here calls a share-immediately
// mutation -- every write is either mode: addToQueue (post/thread/image,
// which DO publish for real if the target channel's queue is unpaused --
// Buffer treats a queued post as live, not draft), saveToDraft: true
// (draft/draft-image, which never leave draft state), or createIdea
// (idea, which has no channel attached at all). See `bfr --help` for which
// verb is which.
//
// The Buffer client lives in the bufferclient package, which has no
// project-specific defaults of its own, so it can be imported and reused by
// other tools without carrying this CLI along.
//
// <channel> is a channel id or a channel name from 'bfr channels' (cached,
// case-insensitive, to .bfr-channels.json next to the repo root by default
// -- override with BUFFER_CACHE_FILE).
//
// Token: BUFFER_API_KEY from the environment, or from a .env file at the
// repo root. Never a flag, never echoed, never committed.
package main

import (
	"fmt"
	"os"
)

const usageText = `Usage: bfr <command> [args]

  bfr channels                            list channel ids, cache to .bfr-channels.json
  bfr idea   <file.md>                    ideas board, no channel attached -- never posts
  bfr draft  <channel> <file.md>          draft ON the channel (saveToDraft) -- never posts
  bfr schedule <channel> <file.md> <ISO8601-datetime>   schedule for a future time (customScheduled) -- will post at that time
  bfr post   <channel> <file.md>          PUBLISHES LIVE -- queues to the channel, will post
  bfr thread <channel> <file.md>          PUBLISHES LIVE -- '---' blocks become a thread, will post
  bfr image  <channel> <file.md> <path>   PUBLISHES LIVE -- drive-upload, attach, queue, will post
  bfr draft-image <channel> <file.md> <path>   draft ON the channel WITH an image -- never posts
  bfr attach-image <post-id> <url>        attach an image to an EXISTING draft/post -- never creates a
                                           new one, never changes text/status/schedule
  bfr show   <post-id>                    read a post/draft back -- status, channel, text, assets
  bfr list   [channel]                    list drafts -- id, status, channel, text, image attached. Read-only
  bfr delete <post-id>                    PERMANENTLY deletes a post/draft -- irreversible, no undo
  bfr version                             print version, commit and build date

Both real channels have an unpaused queue, so post/thread/image publish for real -- there is no
"safe" queue state for them. idea and draft are the only two verbs that never reach an audience.

<channel> is a channel id or a channel name from 'bfr channels' (cached, case-insensitive).
Token: export BUFFER_API_KEY, or set it in this repo's .env. Never a flag.
Image verbs also require BUFFER_DRIVE_ACCOUNT (a gog-authenticated Drive account email) and the
gog and sips CLIs.
`

func failUsage() {
	fmt.Print(usageText)
	os.Exit(1)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Print(usageText)
		return
	}
	ensureEnv()
	cmd, rest := args[0], args[1:]

	switch cmd {
	case "channels":
		cmdChannels()
	case "idea":
		if len(rest) < 1 {
			failUsage()
		}
		cmdIdea(rest[0])
	case "draft":
		if len(rest) < 2 {
			failUsage()
		}
		cmdDraft(rest[0], rest[1])
	case "schedule":
		if len(rest) < 3 {
			failUsage()
		}
		cmdSchedule(rest[0], rest[1], rest[2])
	case "post":
		if len(rest) < 2 {
			failUsage()
		}
		cmdPost(rest[0], rest[1])
	case "thread":
		if len(rest) < 2 {
			failUsage()
		}
		cmdThread(rest[0], rest[1])
	case "image":
		if len(rest) < 3 {
			failUsage()
		}
		cmdImage(rest[0], rest[1], rest[2])
	case "draft-image":
		if len(rest) < 3 {
			failUsage()
		}
		cmdDraftImage(rest[0], rest[1], rest[2])
	case "attach-image":
		if len(rest) < 2 {
			failUsage()
		}
		cmdAttachImage(rest[0], rest[1])
	case "show":
		if len(rest) < 1 {
			failUsage()
		}
		cmdShow(rest[0])
	case "list":
		channelArg := ""
		if len(rest) >= 1 {
			channelArg = rest[0]
		}
		cmdList(channelArg)
	case "delete":
		if len(rest) < 1 {
			failUsage()
		}
		cmdDelete(rest[0])
	case "version", "-v", "--version":
		cmdVersion()
	case "", "-h", "--help", "help":
		fmt.Print(usageText)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Print(usageText)
		os.Exit(1)
	}
}
