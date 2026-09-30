package bufferclient

import (
	"encoding/json"
	"fmt"
	"sort"
)

// MaxSentStats caps how many sent posts ListSent will return. Buffer's
// posts connection is fetched in a single page of this size.
const MaxSentStats = 50

// Metric is one post-level analytics value Buffer reports for a sent post.
// Type is the machine name (reactions, comments, impressions, reach,
// engagementRate); Unit is "count" or "percentage".
type Metric struct {
	Type  string  `json:"type"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// PostStats is the read-only analytics view of a post. MetricsUpdatedAt is
// when Buffer last refreshed the numbers, which can lag well behind SentAt.
type PostStats struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	SentAt  string `json:"sentAt"`
	Text    string `json:"text"`
	Channel struct {
		Name    string `json:"name"`
		Service string `json:"service"`
	} `json:"channel"`
	Metrics          []Metric `json:"metrics"`
	MetricsUpdatedAt string   `json:"metricsUpdatedAt"`
}

// Value returns the metric of the given type, and whether Buffer returned
// it at all. A metric Buffer does not return (clicks on LinkedIn, or
// impressions on a post too new to have been measured) is absent, never
// silently zero.
func (s PostStats) Value(metricType string) (float64, bool) {
	for _, m := range s.Metrics {
		if m.Type == metricType {
			return m.Value, true
		}
	}
	return 0, false
}

const statsFields = `id status sentAt text channel { name service } metrics { type name value unit } metricsUpdatedAt`

// GetStats reads one post's metrics. Read-only query.
func (c *Client) GetStats(postID string) (*PostStats, []byte, error) {
	resp, err := c.Raw(
		`query($id: PostId!) { post(input:{id:$id}) { `+statsFields+` } }`,
		map[string]string{"id": postID},
	)
	if err != nil {
		return nil, nil, err
	}
	var parsed struct {
		Data struct {
			Post *PostStats `json:"post"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil || parsed.Data.Post == nil {
		return nil, resp, fmt.Errorf("post not found, or query failed")
	}
	return parsed.Data.Post, resp, nil
}

// ListSent returns up to n sent posts with their metrics, newest sentAt
// first. Read-only query. n must be between 1 and MaxSentStats.
func (c *Client) ListSent(orgID string, n int) ([]PostStats, []byte, error) {
	if n < 1 || n > MaxSentStats {
		return nil, nil, fmt.Errorf("count must be between 1 and %d, got %d", MaxSentStats, n)
	}
	vars := map[string]interface{}{
		"input": map[string]interface{}{
			"organizationId": orgID,
			"filter":         map[string]interface{}{"status": []string{"sent"}},
			"sort":           []map[string]string{{"field": "createdAt", "direction": "desc"}},
		},
		"first": MaxSentStats,
	}
	resp, err := c.Raw(
		`query($input: PostsInput!, $first: Int) { posts(input:$input, first:$first) { edges { node { `+statsFields+` } } } }`,
		vars,
	)
	if err != nil {
		return nil, nil, err
	}
	var parsed struct {
		Data struct {
			Posts struct {
				Edges []struct {
					Node PostStats `json:"node"`
				} `json:"edges"`
			} `json:"posts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &parsed); err != nil || parsed.Data.Posts.Edges == nil {
		return nil, resp, fmt.Errorf("posts query failed")
	}
	items := make([]PostStats, 0, len(parsed.Data.Posts.Edges))
	for _, e := range parsed.Data.Posts.Edges {
		items = append(items, e.Node)
	}
	// createdAt order is not send order (a draft made long ago can go out
	// today), so the "last n sent" cut is made on sentAt.
	sort.SliceStable(items, func(i, j int) bool { return items[i].SentAt > items[j].SentAt })
	if len(items) > n {
		items = items[:n]
	}
	return items, resp, nil
}
