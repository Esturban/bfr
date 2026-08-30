// Package bufferclient is a minimal client for Buffer's GraphQL API
// (https://api.buffer.com). It knows nothing about any particular account,
// organization, or channel -- every identifier is passed in by the caller.
// It is deliberately small enough to be lifted into its own module: no
// project-specific paths, no cached defaults, no publish/send behavior of
// its own beyond what the caller explicitly asks for.
package bufferclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

const defaultAPI = "https://api.buffer.com"

// Client talks to the Buffer GraphQL API with a single bearer token.
type Client struct {
	APIURL string
	Token  string
	HTTP   *http.Client
}

// New returns a Client for the given API token. api may be empty, which
// selects the default https://api.buffer.com endpoint.
func New(token, api string) *Client {
	if api == "" {
		api = defaultAPI
	}
	return &Client{APIURL: api, Token: token, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

type gqlRequest struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables,omitempty"`
}

// Raw sends a GraphQL request and returns the raw response body. Callers
// that need a typed result should use one of the methods below; Raw exists
// for BLOCKED-path error reporting, where the original response body is
// shown to the operator rather than reinterpreted.
func (c *Client) Raw(query string, variables interface{}) ([]byte, error) {
	body, err := json.Marshal(gqlRequest{Query: query, Variables: variables})
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, c.APIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", c.APIURL, err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// --- typed calls -----------------------------------------------------------

// OrganizationID resolves the first organization id visible to the token.
func (c *Client) OrganizationID() (string, []byte, error) {
	resp, err := c.Raw(`query { account { organizations { id } } }`, nil)
	if err != nil {
		return "", nil, err
	}
	var parsed struct {
		Data struct {
			Account struct {
				Organizations []struct {
					ID string `json:"id"`
				} `json:"organizations"`
			} `json:"account"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return "", resp, fmt.Errorf("decoding organizations response: %w", err)
	}
	if len(parsed.Data.Account.Organizations) == 0 {
		return "", resp, fmt.Errorf("could not resolve organizationId")
	}
	return parsed.Data.Account.Organizations[0].ID, resp, nil
}

// Channel is one Buffer channel (a connected social account).
type Channel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Service       string `json:"service"`
	Avatar        string `json:"avatar"`
	IsQueuePaused bool   `json:"isQueuePaused"`
}

// Channels lists every channel on the given organization.
func (c *Client) Channels(orgID string) ([]Channel, []byte, error) {
	resp, err := c.Raw(
		`query($orgId: OrganizationId!) { channels(input:{organizationId:$orgId}) { id name service avatar isQueuePaused } }`,
		map[string]string{"orgId": orgID},
	)
	if err != nil {
		return nil, nil, err
	}
	var parsed struct {
		Data struct {
			Channels []Channel `json:"channels"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil || parsed.Data.Channels == nil {
		return nil, resp, fmt.Errorf("channels query failed")
	}
	return parsed.Data.Channels, resp, nil
}

// PostInput is the shape createPost accepts. Mode is "addToQueue" (queue or
// draft) or "customScheduled" (schedule for a specific dueAt) -- this client
// never calls a share-immediately mutation.
type PostInput struct {
	Text           string      `json:"text"`
	ChannelID      string      `json:"channelId"`
	SchedulingType string      `json:"schedulingType"`
	Mode           string      `json:"mode"`
	SaveToDraft    bool        `json:"saveToDraft,omitempty"`
	DueAt          string      `json:"dueAt,omitempty"`
	Metadata       interface{} `json:"metadata,omitempty"`
	Assets         interface{} `json:"assets,omitempty"`
}

const createPostQuery = `mutation($input: CreatePostInput!) { createPost(input: $input) { __typename ... on PostActionSuccess { post { id status } } ... on NotFoundError { message } ... on UnauthorizedError { message } ... on UnexpectedError { message } ... on RestProxyError { code message link } ... on LimitReachedError { message } ... on InvalidInputError { message } } }`

// PostResult is the union createPost/it returns, flattened.
type PostResult struct {
	Typename string
	PostID   string
	Status   string
	Message  string
}

// CreatePost runs the createPost mutation and returns the raw response
// alongside the parsed union result, so a BLOCKED caller can show the
// operator the exact body Buffer returned.
func (c *Client) CreatePost(input PostInput) (PostResult, []byte, error) {
	resp, err := c.Raw(createPostQuery, map[string]interface{}{"input": input})
	if err != nil {
		return PostResult{}, nil, err
	}
	var parsed struct {
		Data struct {
			CreatePost struct {
				Typename string `json:"__typename"`
				Post     struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"post"`
				Message string `json:"message"`
			} `json:"createPost"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return PostResult{}, resp, fmt.Errorf("decoding createPost response: %w", err)
	}
	return PostResult{
		Typename: parsed.Data.CreatePost.Typename,
		PostID:   parsed.Data.CreatePost.Post.ID,
		Status:   parsed.Data.CreatePost.Post.Status,
		Message:  parsed.Data.CreatePost.Message,
	}, resp, nil
}

// EditPostInput is the shape editPost accepts. Only ID is required --
// every other field is optional and omitted when not set, so a caller that
// only wants to attach an asset (the attach-image verb's whole purpose)
// sends nothing else: no text, no mode, no schedulingType. editPost is the
// sibling mutation to createPost that changes an EXISTING post/draft rather
// than creating a new one. Mode/SchedulingType/DueAt exist on Buffer's real
// EditPostInput (confirmed via __type introspection) alongside Text --
// setting only those three, on an existing id, is what lets schedule give a
// draft a date without touching its text, assets, or channel.
type EditPostInput struct {
	ID             string      `json:"id"`
	Text           *string     `json:"text,omitempty"`
	Assets         interface{} `json:"assets,omitempty"`
	Mode           *string     `json:"mode,omitempty"`
	SchedulingType *string     `json:"schedulingType,omitempty"`
	DueAt          *string     `json:"dueAt,omitempty"`
	SaveToDraft    *bool       `json:"saveToDraft,omitempty"`
}

const editPostQuery = `mutation($input: EditPostInput!) { editPost(input: $input) { __typename ... on PostActionSuccess { post { id status } } ... on NotFoundError { message } ... on UnauthorizedError { message } ... on UnexpectedError { message } ... on RestProxyError { code message link } ... on LimitReachedError { message } ... on InvalidInputError { message } } }`

// EditPost runs the editPost mutation and returns the raw response
// alongside the parsed union result, mirroring CreatePost -- same
// PostResult shape, same "typename tells you which union branch fired"
// contract, so callers already handling CreatePost/handlePostResult-style
// results can handle EditPost's the same way.
func (c *Client) EditPost(input EditPostInput) (PostResult, []byte, error) {
	resp, err := c.Raw(editPostQuery, map[string]interface{}{"input": input})
	if err != nil {
		return PostResult{}, nil, err
	}
	var parsed struct {
		Data struct {
			EditPost struct {
				Typename string `json:"__typename"`
				Post     struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"post"`
				Message string `json:"message"`
			} `json:"editPost"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return PostResult{}, resp, fmt.Errorf("decoding editPost response: %w", err)
	}
	return PostResult{
		Typename: parsed.Data.EditPost.Typename,
		PostID:   parsed.Data.EditPost.Post.ID,
		Status:   parsed.Data.EditPost.Post.Status,
		Message:  parsed.Data.EditPost.Message,
	}, resp, nil
}

const createIdeaQuery = `mutation($input: CreateIdeaInput!) { createIdea(input: $input) { __typename ... on Idea { id } ... on IdeaResponse { idea { id } } ... on InvalidInputError { message } ... on UnauthorizedError { message } ... on UnexpectedError { message } ... on LimitReachedError { message } } }`

// IdeaResult is the union createIdea returns, flattened.
type IdeaResult struct {
	Typename string
	IdeaID   string
	Message  string
}

// CreateIdea puts text on the ideas board -- no channel attached, never posts.
func (c *Client) CreateIdea(orgID, text string) (IdeaResult, []byte, error) {
	vars := map[string]interface{}{
		"input": map[string]interface{}{
			"organizationId": orgID,
			"content":        map[string]string{"text": text},
		},
	}
	resp, err := c.Raw(createIdeaQuery, vars)
	if err != nil {
		return IdeaResult{}, nil, err
	}
	var parsed struct {
		Data struct {
			CreateIdea struct {
				Typename string `json:"__typename"`
				ID       string `json:"id"`
				Idea     struct {
					ID string `json:"id"`
				} `json:"idea"`
				Message string `json:"message"`
			} `json:"createIdea"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return IdeaResult{}, resp, fmt.Errorf("decoding createIdea response: %w", err)
	}
	id := parsed.Data.CreateIdea.ID
	if id == "" {
		id = parsed.Data.CreateIdea.Idea.ID
	}
	return IdeaResult{Typename: parsed.Data.CreateIdea.Typename, IdeaID: id, Message: parsed.Data.CreateIdea.Message}, resp, nil
}

// Asset is one attached asset on a post, as returned by Get/List.
type Asset struct {
	MimeType  string `json:"mimeType"`
	Source    string `json:"source"`
	Thumbnail string `json:"thumbnail"`
}

// PostDetail is the read shape returned by Get (bfr show).
type PostDetail struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	DueAt          string `json:"dueAt"`
	SchedulingType string `json:"schedulingType"`
	Channel        struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Service string `json:"service"`
	} `json:"channel"`
	Text   string  `json:"text"`
	Assets []Asset `json:"assets"`
}

// Get reads a single post/draft back by id. Read-only.
func (c *Client) Get(postID string) (*PostDetail, []byte, error) {
	resp, err := c.Raw(
		`query($id: PostId!) { post(input:{id:$id}) { id status dueAt schedulingType channel { id name service } text assets { mimeType source thumbnail } } }`,
		map[string]string{"id": postID},
	)
	if err != nil {
		return nil, nil, err
	}
	var parsed struct {
		Data struct {
			Post *PostDetail `json:"post"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil || parsed.Data.Post == nil {
		return nil, resp, fmt.Errorf("post not found, or query failed")
	}
	return parsed.Data.Post, resp, nil
}

// ListItem is one row from List. DueAt and SchedulingType are only
// populated for scheduled posts -- a draft has no due time yet. CMO-2558:
// ten posts sat scheduled at 20:00 UTC (23:00 Riyadh) with no way to see
// them, because List only ever queried status:[draft]. It now also queries
// status:[scheduled] so the same command surfaces both.
type ListItem struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Text           string `json:"text"`
	CreatedAt      string `json:"createdAt"`
	DueAt          string `json:"dueAt"`
	SchedulingType string `json:"schedulingType"`
	Channel        struct {
		Name    string `json:"name"`
		Service string `json:"service"`
	} `json:"channel"`
	Assets []struct {
		MimeType string `json:"mimeType"`
		Source   string `json:"source"`
	} `json:"assets"`
}

// List returns up to 50 draft and scheduled posts, optionally filtered to
// one channel id. Sorted by dueAt where a post has one (scheduled posts,
// soonest first) and otherwise by createdAt (drafts have no due time), so a
// caller scanning the output sees the schedule in the order it will
// actually fire, with drafts trailing in creation order.
func (c *Client) List(orgID string, channelID string) ([]ListItem, []byte, error) {
	filter := map[string]interface{}{"status": []string{"draft", "scheduled"}}
	if channelID != "" {
		filter["channelIds"] = []string{channelID}
	}
	vars := map[string]interface{}{
		"input": map[string]interface{}{
			"organizationId": orgID,
			"filter":         filter,
		},
	}
	resp, err := c.Raw(
		`query($input: PostsInput!) { posts(input:$input, first:50) { edges { node { id status text createdAt dueAt schedulingType channel { name service } assets { mimeType source } } } } }`,
		vars,
	)
	if err != nil {
		return nil, nil, err
	}
	var parsed struct {
		Data struct {
			Posts struct {
				Edges []struct {
					Node ListItem `json:"node"`
				} `json:"edges"`
			} `json:"posts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil || parsed.Data.Posts.Edges == nil {
		return nil, resp, fmt.Errorf("posts query failed")
	}
	items := make([]ListItem, 0, len(parsed.Data.Posts.Edges))
	for _, e := range parsed.Data.Posts.Edges {
		items = append(items, e.Node)
	}
	sort.Slice(items, func(i, j int) bool { return sortKey(items[i]) < sortKey(items[j]) })
	return items, resp, nil
}

// sortKey orders a scheduled post by its dueAt and everything else by when
// it was created. A draft can carry a stale dueAt left over from a prior
// schedule that was reset, so DueAt is only trusted as a sort key when
// status is actually "scheduled".
func sortKey(it ListItem) string {
	if it.Status == "scheduled" && it.DueAt != "" {
		return it.DueAt
	}
	return it.CreatedAt
}

const deletePostQuery = `mutation($input: DeletePostInput!) { deletePost(input: $input) { __typename ... on DeletePostSuccess { id } ... on VoidMutationError { message } } }`

// DeleteResult is the union deletePost returns, flattened.
type DeleteResult struct {
	Typename string
	PostID   string
	Message  string
}

// Delete permanently removes a post/draft. Irreversible -- Buffer has no
// undo for this mutation.
func (c *Client) Delete(postID string) (DeleteResult, []byte, error) {
	resp, err := c.Raw(deletePostQuery, map[string]interface{}{"input": map[string]string{"id": postID}})
	if err != nil {
		return DeleteResult{}, nil, err
	}
	var parsed struct {
		Data struct {
			DeletePost struct {
				Typename string `json:"__typename"`
				ID       string `json:"id"`
				Message  string `json:"message"`
			} `json:"deletePost"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil {
		return DeleteResult{}, resp, fmt.Errorf("decoding deletePost response: %w", err)
	}
	return DeleteResult{
		Typename: parsed.Data.DeletePost.Typename,
		PostID:   parsed.Data.DeletePost.ID,
		Message:  parsed.Data.DeletePost.Message,
	}, resp, nil
}
