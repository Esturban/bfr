package bufferclient

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	if _, _, err := c.ListSent("org1", MaxSentStats+1); err == nil {
		t.Fatal("n above the cap must be rejected")
	}
}
