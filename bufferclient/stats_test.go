package bufferclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// newStatsServer returns a Client wired to a fake Buffer API that answers
// every request with body, and records the last request body it saw.
func newStatsServer(t *testing.T, body string) (*Client, *string) {
	t.Helper()
	var lastReq string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		lastReq = string(b)
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization header = %q, want bearer token", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return New("test-token", srv.URL), &lastReq
}

const sampleMetrics = `[
 {"type":"reactions","name":"Reactions","value":4,"unit":"count"},
 {"type":"comments","name":"Comments","value":1,"unit":"count"},
 {"type":"engagementRate","name":"Engagement rate","value":1.25,"unit":"percentage"},
 {"type":"impressions","name":"Impressions","value":321,"unit":"count"},
 {"type":"reach","name":"Reach","value":216,"unit":"count"}]`

func TestPostStatsValue(t *testing.T) {
	var s PostStats
	if err := json.Unmarshal([]byte(`{"id":"p1","metrics":`+sampleMetrics+`}`), &s); err != nil {
		t.Fatal(err)
	}
	if v, ok := s.Value("impressions"); !ok || v != 321 {
		t.Fatalf("impressions = %v,%v want 321,true", v, ok)
	}
	if v, ok := s.Value("engagementRate"); !ok || v != 1.25 {
		t.Fatalf("engagementRate = %v,%v want 1.25,true", v, ok)
	}
	if _, ok := s.Value("clicks"); ok {
		t.Fatal("clicks must report not-present when Buffer does not return it")
	}
}

func TestGetStatsParsesMetrics(t *testing.T) {
	c, req := newStatsServer(t, `{"data":{"post":{"id":"p1","status":"sent","sentAt":"2026-09-01T10:01:14.278Z","text":"hi","channel":{"name":"EV","service":"linkedin"},"metrics":`+sampleMetrics+`,"metricsUpdatedAt":"2026-09-29T19:55:06.774Z"}}}`)
	s, _, err := c.GetStats("p1")
	if err != nil {
		t.Fatalf("GetStats error: %v", err)
	}
	if s.ID != "p1" || s.Status != "sent" || len(s.Metrics) != 5 {
		t.Fatalf("unexpected stats: %+v", s)
	}
	if strings.Contains(*req, "mutation") {
		t.Fatalf("GetStats must be a read-only query, sent: %s", *req)
	}
}

func TestGetStatsNotFound(t *testing.T) {
	c, _ := newStatsServer(t, `{"data":{"post":null},"errors":[{"message":"not found"}]}`)
	if _, resp, err := c.GetStats("nope"); err == nil || len(resp) == 0 {
		t.Fatalf("want error with raw response, got err=%v resp=%q", err, resp)
	}
}

func TestListSentNewestFirstAndCapped(t *testing.T) {
	body := `{"data":{"posts":{"edges":[
	 {"node":{"id":"old","status":"sent","sentAt":"2026-08-01T10:00:00Z","metrics":[]}},
	 {"node":{"id":"new","status":"sent","sentAt":"2026-09-30T10:00:00Z","metrics":[]}},
	 {"node":{"id":"mid","status":"sent","sentAt":"2026-09-01T10:00:00Z","metrics":[]}}]}}}`
	c, req := newStatsServer(t, body)
	items, _, err := c.ListSent("org1", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "new" || items[1].ID != "mid" {
		t.Fatalf("want [new mid], got %+v", items)
	}
	if !strings.Contains(*req, "sent") || strings.Contains(*req, "mutation") {
		t.Fatalf("ListSent must query status sent and never mutate, sent: %s", *req)
	}
}

func TestListSentRejectsBadCount(t *testing.T) {
	c, _ := newStatsServer(t, `{}`)
	if _, _, err := c.ListSent("org1", 0); err == nil {
		t.Fatal("n=0 must be rejected")
	}
}

// newPagedServer serves total sent posts in pages, honouring the "after"
// cursor and "first" page size, and records every request body.
func newPagedServer(t *testing.T, total int) (*Client, *[]string) {
	t.Helper()
	var reqs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		reqs = append(reqs, string(b))
		var req struct {
			Variables struct {
				First int    `json:"first"`
				After string `json:"after"`
			} `json:"variables"`
		}
		_ = json.Unmarshal(b, &req)
		start := 0
		if req.Variables.After != "" {
			start, _ = strconv.Atoi(strings.TrimPrefix(req.Variables.After, "c"))
		}
		end := start + req.Variables.First
		if end > total {
			end = total
		}
		var edges []string
		for i := start; i < end; i++ {
			edges = append(edges, fmt.Sprintf(`{"node":{"id":"p%03d","status":"sent","sentAt":"2026-01-01T00:%02d:%02dZ","metrics":[]}}`, i, i/60, i%60))
		}
		_, _ = fmt.Fprintf(w, `{"data":{"posts":{"edges":[%s],"pageInfo":{"hasNextPage":%t,"endCursor":"c%d"}}}}`,
			strings.Join(edges, ","), end < total, end)
	}))
	t.Cleanup(srv.Close)
	return New("test-token", srv.URL), &reqs
}

func TestListSentPagesPastOnePage(t *testing.T) {
	c, reqs := newPagedServer(t, 61)
	items, _, err := c.ListSent("org1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 61 {
		t.Fatalf("want all 61 posts, got %d", len(items))
	}
	if len(*reqs) != 2 {
		t.Fatalf("want 2 page requests, got %d", len(*reqs))
	}
	seen := map[string]bool{}
	for _, it := range items {
		if seen[it.ID] {
			t.Fatalf("duplicate post %s across pages", it.ID)
		}
		seen[it.ID] = true
	}
	for _, r := range *reqs {
		if strings.Contains(r, "mutation") {
			t.Fatalf("paging must stay read-only, sent: %s", r)
		}
	}
}

func TestListSentStopsPagingOnceEnough(t *testing.T) {
	c, reqs := newPagedServer(t, 500)
	items, _, err := c.ListSent("org1", 60)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 60 || len(*reqs) != 2 {
		t.Fatalf("want 60 items in 2 requests, got %d in %d", len(items), len(*reqs))
	}
}

func TestListSentStopsWhenCursorDoesNotAdvance(t *testing.T) {
	body := `{"data":{"posts":{"edges":[{"node":{"id":"a","sentAt":"2026-01-01T00:00:00Z"}}],"pageInfo":{"hasNextPage":true,"endCursor":""}}}}`
	c, _ := newStatsServer(t, body)
	items, _, err := c.ListSent("org1", 100)
	if err != nil || len(items) != 1 {
		t.Fatalf("want 1 item and no infinite loop, got %d, %v", len(items), err)
	}
}
