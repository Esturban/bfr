package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Esturban/bfr/bufferclient"
)

// blocked prints a BLOCKED line to stderr and exits 1 -- same contract as
// the bash tool's `echo "BLOCKED: ..." >&2; exit 1`.
func blocked(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "BLOCKED: "+format+"\n", args...)
	os.Exit(1)
}

// blockedResponse is blocked() plus a raw response dump, for the "unexpected
// response" paths where the original tool showed the operator the exact
// body Buffer returned instead of a summarized error.
func blockedResponse(msg string, resp []byte) {
	fmt.Fprintf(os.Stderr, "BLOCKED: %s. Response:\n%s\n", msg, resp)
	os.Exit(1)
}

func newClient() *bufferclient.Client {
	tok, err := apiToken()
	if err != nil {
		blocked("%s", err)
	}
	return bufferclient.New(tok, "")
}

// --- read verbs --------------------------------------------------------

func cmdChannels() {
	c := newClient()
	org, resp, err := c.OrganizationID()
	if err != nil {
		blocked("%s", err)
	}
	if org == "" {
		blockedResponse("could not resolve organizationId", resp)
	}
	channels, resp, err := c.Channels(org)
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	if err := writeCache(org, channels); err != nil {
		blocked("could not write cache: %s", err)
	}
	fmt.Printf("Cached %d channel(s) to %s\n", len(channels), cachePath())
	for _, ch := range channels {
		fmt.Printf("%s\t%s\t%s\tqueuePaused=%t\n", ch.ID, ch.Name, ch.Service, ch.IsQueuePaused)
	}
}

func cmdShow(postID string, full bool) {
	c := newClient()
	post, resp, err := c.Get(postID)
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	fmt.Printf("id:      %s\n", post.ID)
	fmt.Printf("status:  %s\n", post.Status)
	if post.DueAt != "" {
		fmt.Printf("dueAt:   %s\n", post.DueAt)
	}
	fmt.Printf("channel: %s (%s)\n", post.Channel.Name, post.Channel.Service)
	if full {
		fmt.Printf("text:    %s\n", post.Text)
	} else {
		fmt.Printf("text:    %s\n", truncate(post.Text, 100))
	}
	if len(post.Assets) == 0 {
		fmt.Println("assets:  none attached")
		return
	}
	for _, a := range post.Assets {
		fmt.Printf("asset:   mimeType=%s source=%s\n", a.MimeType, a.Source)
	}
}

func cmdList(channelArg string) {
	c := newClient()
	org, err := orgID(c)
	if err != nil {
		blocked("%s", err)
	}
	channelID := ""
	if channelArg != "" {
		channelID, err = resolveChannel(channelArg)
		if err != nil {
			blocked("%s", err)
		}
	}
	items, resp, err := c.List(org, channelID)
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	for _, it := range items {
		imageFlag := "image=NO"
		if len(it.Assets) > 0 {
			imageFlag = "image=yes"
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n", it.ID, it.CreatedAt, it.Status, it.Channel.Name, truncate(it.Text, 50), imageFlag)
	}
}

// --- write verbs (queue / draft, never publish-now) ---------------------

func cmdIdea(file string) {
	text, err := readBody(file)
	if err != nil {
		blocked("%s", err)
	}
	c := newClient()
	org, err := orgID(c)
	if err != nil {
		blocked("%s", err)
	}
	result, resp, err := c.CreateIdea(org, text)
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	switch result.Typename {
	case "Idea", "IdeaResponse":
		fmt.Printf("IDEA CREATED (draft only, not posted): id=%s\n", result.IdeaID)
	case "":
		blockedResponse("unexpected response", resp)
	default:
		msg := result.Message
		if msg == "" {
			msg = "unknown error"
		}
		blocked("(%s): %s", result.Typename, msg)
	}
}

func handlePostResult(result bufferclient.PostResult, resp []byte) {
	switch result.Typename {
	case "PostActionSuccess":
		fmt.Printf("QUEUED: post id=%s status=%s\n", result.PostID, result.Status)
	case "":
		blockedResponse("unexpected response", resp)
	default:
		msg := result.Message
		if msg == "" {
			msg = "unknown error"
		}
		blocked("(%s): %s", result.Typename, msg)
	}
}

func cmdPost(channelArg, file string) {
	channel, err := resolveChannel(channelArg)
	if err != nil {
		blocked("%s", err)
	}
	text, err := readBody(file)
	if err != nil {
		blocked("%s", err)
	}
	c := newClient()
	result, resp, err := c.CreatePost(bufferclient.PostInput{
		Text: text, ChannelID: channel, SchedulingType: "automatic", Mode: "addToQueue",
	})
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	handlePostResult(result, resp)
}

// cmdThread splits file on lines that are exactly "---" into thread blocks.
// The first non-empty block becomes the post text; the rest become
// metadata.twitter.thread entries. Line-for-line port of cmd_thread's
// accumulate-then-trim loop.
func cmdThread(channelArg, file string) {
	channel, err := resolveChannel(channelArg)
	if err != nil {
		blocked("%s", err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		blocked("file not found: %s", file)
	}

	var firstText string
	haveFirst := false
	var blocks []map[string]string
	var current strings.Builder

	flush := func() {
		trimmed := strings.TrimSpace(current.String())
		current.Reset()
		if trimmed == "" {
			return
		}
		if !haveFirst {
			firstText = trimmed
			haveFirst = true
		} else {
			blocks = append(blocks, map[string]string{"text": trimmed})
		}
	}

	for _, line := range strings.Split(string(data), "\n") {
		if line == "---" {
			flush()
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
	}
	flush()

	if !haveFirst {
		blocked("empty thread file")
	}

	var metadata interface{}
	if len(blocks) > 0 {
		metadata = map[string]interface{}{"twitter": map[string]interface{}{"thread": blocks}}
	}

	c := newClient()
	result, resp, err := c.CreatePost(bufferclient.PostInput{
		Text: firstText, ChannelID: channel, SchedulingType: "automatic", Mode: "addToQueue", Metadata: metadata,
	})
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	handlePostResult(result, resp)
}

// handleDraftResult is deliberately separate from handlePostResult: draft
// and draft-image must never be mistaken for the live-publishing verbs
// (post/thread/image), so their success line says DRAFT, never QUEUED, and
// this path does not share code with the proven queue path. Same discipline
// the original bash tool's cmd_draft comment documented.
func handleDraftResult(channelArg string, result bufferclient.PostResult, resp []byte) {
	switch result.Typename {
	case "PostActionSuccess":
		fmt.Printf("DRAFT on %s (not queued, not published): post id=%s status=%s\n", channelArg, result.PostID, result.Status)
	case "":
		blockedResponse("unexpected response", resp)
	default:
		msg := result.Message
		if msg == "" {
			msg = "unknown error"
		}
		blocked("(%s): %s", result.Typename, msg)
	}
}

func cmdDraft(channelArg, file string) {
	channel, err := resolveChannel(channelArg)
	if err != nil {
		blocked("%s", err)
	}
	text, err := readBody(file)
	if err != nil {
		blocked("%s", err)
	}
	c := newClient()
	result, resp, err := c.CreatePost(bufferclient.PostInput{
		Text: text, ChannelID: channel, SchedulingType: "automatic", Mode: "addToQueue", SaveToDraft: true,
	})
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	handleDraftResult(channelArg, result, resp)
}

// handleScheduleResult is deliberately separate from handlePostResult and
// handleDraftResult: schedule must never be mistaken for either -- its
// success line says SCHEDULED, never QUEUED or DRAFT.
func handleScheduleResult(dueAt string, result bufferclient.PostResult, resp []byte) {
	switch result.Typename {
	case "PostActionSuccess":
		if result.Status != "scheduled" {
			blocked("post %s came back status '%s', not 'scheduled' -- editPost returned success but did not actually apply the schedule. Response:\n%s", result.PostID, result.Status, resp)
		}
		fmt.Printf("SCHEDULED for %s: post id=%s status=%s\n", dueAt, result.PostID, result.Status)
	case "":
		blockedResponse("unexpected response", resp)
	default:
		msg := result.Message
		if msg == "" {
			msg = "unknown error"
		}
		blocked("(%s): %s", result.Typename, msg)
	}
}

// cmdSchedule gives an EXISTING draft a future scheduled date via editPost,
// setting only mode/schedulingType/dueAt -- never creates a new post, and
// never touches the draft's text, assets, or channel. Refuses anything that
// is not currently a draft (queued or already-scheduled posts are not safe
// to reschedule here), and refuses a past or malformed datetime, before any
// network call. This replaces the earlier file-based signature
// (bfr schedule <channel> <file.md> <datetime>, which created a NEW post)
// per EV, 2026-08-29: scheduling an EV-approved draft must never mean
// re-uploading it -- that file-based path had never been used, so nothing
// depends on the old signature.
func cmdSchedule(postID, datetime string) {
	if strings.TrimSpace(postID) == "" {
		blocked("post id is required")
	}
	dueAt, err := time.Parse(time.RFC3339, datetime)
	if err != nil {
		blocked("invalid datetime %q, expected ISO 8601 (RFC3339), e.g. 2026-09-01T14:00:00Z: %s", datetime, err)
	}
	if !dueAt.After(time.Now()) {
		blocked("datetime %q is not in the future", datetime)
	}

	c := newClient()
	before, resp, err := c.Get(postID)
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	if before.Status != "draft" {
		blocked("post %s has status '%s', not 'draft' -- schedule refuses to touch anything that is not currently a draft.", postID, before.Status)
	}

	mode := "customScheduled"
	schedulingType := "automatic"
	saveToDraft := false
	dueAtStr := dueAt.UTC().Format(time.RFC3339)
	// editPost has two undocumented requirements, found by live introspection
	// and trial (CMO-2424): (1) InvalidInputError "Post must have either text
	// or media" fires unless text is present in the submitted input, even
	// though the post already has both stored -- echoing back the unchanged
	// text (read via the preflight Get above) satisfies it without altering
	// content; (2) omitting saveToDraft leaves status silently unchanged at
	// "draft" (PostActionSuccess, no error, but nothing actually scheduled)
	// -- saveToDraft: false must be sent explicitly to move a draft to
	// "scheduled".
	result, resp, err := c.EditPost(bufferclient.EditPostInput{
		ID: postID, Text: &before.Text, Mode: &mode, SchedulingType: &schedulingType, DueAt: &dueAtStr, SaveToDraft: &saveToDraft,
	})
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	handleScheduleResult(dueAtStr, result, resp)
}

// cmdAttachImage attaches an image to an EXISTING post/draft via the
// editPost mutation. Unlike image/draft-image, it never creates a new
// post and never touches text, status, schedule, or channel -- the
// editPost request sends ONLY id + assets. url must already be a public,
// Buffer-hotlink-safe URL (GitHub raw works, Google Drive's usercontent
// download link works -- see driveUploadAndShare above for why Drive is
// used elsewhere); this verb does no upload of its own, so a local file
// path is rejected rather than silently failing at Buffer's end.
func cmdAttachImage(postID, url string) {
	if strings.TrimSpace(postID) == "" {
		blocked("post id is required")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		blocked("not a public image URL (must start with http:// or https://): %s -- use 'bfr image'/'bfr draft-image' to upload a local file", url)
	}
	assets := []map[string]interface{}{{"image": map[string]string{"url": url}}}

	c := newClient()
	result, resp, err := c.EditPost(bufferclient.EditPostInput{ID: postID, Assets: assets})
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	switch result.Typename {
	case "PostActionSuccess":
		fmt.Printf("IMAGE ATTACHED: post id=%s status=%s\n", result.PostID, result.Status)
	case "":
		blockedResponse("unexpected response", resp)
	default:
		msg := result.Message
		if msg == "" {
			msg = "unknown error"
		}
		blocked("(%s): %s", result.Typename, msg)
	}
}

// cmdUpdate replaces an EXISTING draft's text via editPost, sending only
// id + text so status/schedule/channel/assets are never touched. Refuses
// to run unless a preflight Get shows status "draft" -- queued or
// published posts are not safe to edit in place. This verb existed on an
// earlier branch (a071493) that was never actually merged into dev; this
// re-adds it after CMO-2413 found the gap.
func cmdUpdate(postID, file string) {
	if strings.TrimSpace(postID) == "" {
		blocked("post id is required")
	}
	text, err := readBody(file)
	if err != nil {
		blocked("%s", err)
	}

	c := newClient()
	before, resp, err := c.Get(postID)
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	if before.Status != "draft" {
		blocked("post %s has status '%s', not 'draft' -- update refuses to touch anything that is queued or published. Only a draft is safe to edit in place.", postID, before.Status)
	}

	result, resp, err := c.EditPost(bufferclient.EditPostInput{ID: postID, Text: &text})
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	switch result.Typename {
	case "PostActionSuccess":
		if result.Status != "draft" {
			blocked("post %s is now status '%s', not 'draft' -- it changed between the preflight check and this edit. The edit was applied; verify it did not land on something no longer safe to touch.", result.PostID, result.Status)
		}
		fmt.Printf("UPDATED: post id=%s status=%s\n", result.PostID, result.Status)
	case "":
		blockedResponse("unexpected response", resp)
	default:
		msg := result.Message
		if msg == "" {
			msg = "unknown error"
		}
		blocked("(%s): %s", result.Typename, msg)
	}
}

func cmdDelete(postID string) {
	c := newClient()
	result, resp, err := c.Delete(postID)
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	switch result.Typename {
	case "DeletePostSuccess":
		fmt.Printf("DELETED: post id=%s\n", result.PostID)
	case "":
		blockedResponse("unexpected response", resp)
	default:
		msg := result.Message
		if msg == "" {
			msg = "unknown error"
		}
		blocked("(%s): %s", result.Typename, msg)
	}
}

// --- image verbs (Drive round-trip, then queue/draft) --------------------

// driveAccount resolves which gog-authenticated Drive account to upload
// through. Unlike the bash tool, this has NO built-in default: the original
// default was one specific person's email address, which is exactly the
// kind of account-specific literal this port must not carry. Set
// BUFFER_DRIVE_ACCOUNT explicitly.
func driveAccount() string {
	acct := os.Getenv("BUFFER_DRIVE_ACCOUNT")
	if acct == "" {
		blocked("BUFFER_DRIVE_ACCOUNT not set. Image verbs upload through a gog-authenticated Drive account -- export the account email to use.")
	}
	return acct
}

// toRealJPEG converts src to actual JPEG bytes via sips (macOS-native),
// because Buffer's ImageAssetInput has no mimeType field and silently
// assumes every asset is JPEG. Same fix as the bash tool's to_real_jpeg,
// same reasoning: this converts an already-generated image's file format
// for delivery, it does not generate or draw anything.
func toRealJPEG(src string) string {
	if _, err := exec.LookPath("sips"); err != nil {
		blocked("sips is required to convert to JPEG for Buffer upload")
	}
	out := strings.TrimSuffix(src, filepath.Ext(src)) + ".bfr-upload.jpg"
	cmd := exec.Command("sips", "-s", "format", "jpeg", src, "--out", out)
	if err := cmd.Run(); err != nil {
		blocked("sips failed converting %s: %s", src, err)
	}
	if _, err := os.Stat(out); err != nil {
		blocked("sips did not produce %s from %s", out, src)
	}
	return out
}

func driveUploadAndShare(path string) string {
	if _, err := exec.LookPath("gog"); err != nil {
		blocked("gog CLI is required for image verbs")
	}
	account := driveAccount()
	upload := toRealJPEG(path)
	fmt.Fprintf(os.Stderr, "Uploading %s to Drive (%s)...\n", upload, account)

	out, err := exec.Command("gog", "drive", "upload", upload, "--json", "-a", account).Output()
	if err != nil {
		blocked("gog drive upload failed: %s", err)
	}
	var parsed struct {
		ID   string `json:"id"`
		File struct {
			ID string `json:"id"`
		} `json:"file"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		blocked("gog drive upload did not return valid JSON:\n%s", out)
	}
	fileID := parsed.File.ID
	if fileID == "" {
		fileID = parsed.ID
	}
	if fileID == "" {
		blocked("gog drive upload did not return a file id:\n%s", out)
	}

	shareCmd := exec.Command("gog", "drive", "share", fileID, "--to", "anyone", "--role", "reader", "--force", "-a", account)
	shareCmd.Stderr = os.Stderr
	if err := shareCmd.Run(); err != nil {
		blocked("gog drive share failed: %s", err)
	}

	return fmt.Sprintf("https://drive.usercontent.google.com/download?id=%s&export=view", fileID)
}

func cmdImage(channelArg, file, path string) {
	if _, err := os.Stat(path); err != nil {
		blocked("image file not found: %s", path)
	}
	channel, err := resolveChannel(channelArg)
	if err != nil {
		blocked("%s", err)
	}
	text, err := readBody(file)
	if err != nil {
		blocked("%s", err)
	}
	url := driveUploadAndShare(path)
	assets := []map[string]interface{}{{"image": map[string]string{"url": url}}}

	c := newClient()
	result, resp, err := c.CreatePost(bufferclient.PostInput{
		Text: text, ChannelID: channel, SchedulingType: "automatic", Mode: "addToQueue", Assets: assets,
	})
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	handlePostResult(result, resp)
	fmt.Printf("Image asset URL: %s\n", url)
}

func cmdDraftImage(channelArg, file, path string) {
	if _, err := os.Stat(path); err != nil {
		blocked("image file not found: %s", path)
	}
	channel, err := resolveChannel(channelArg)
	if err != nil {
		blocked("%s", err)
	}
	text, err := readBody(file)
	if err != nil {
		blocked("%s", err)
	}
	url := driveUploadAndShare(path)
	assets := []map[string]interface{}{{"image": map[string]string{"url": url}}}

	c := newClient()
	result, resp, err := c.CreatePost(bufferclient.PostInput{
		Text: text, ChannelID: channel, SchedulingType: "automatic", Mode: "addToQueue", SaveToDraft: true, Assets: assets,
	})
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	handleDraftResult(channelArg, result, resp)
	fmt.Printf("Image asset URL: %s\n", url)
}
