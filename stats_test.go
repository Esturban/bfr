package main

import (
	"strings"
	"testing"

	"github.com/Esturban/bfr/bufferclient"
)

func samplePost() bufferclient.PostStats {
	return bufferclient.PostStats{
		ID:     "p1",
		SentAt: "2026-09-01T10:01:14Z",
		Text:   "line one\nline two",
		Metrics: []bufferclient.Metric{
			{Type: "impressions", Value: 321, Unit: "count"},
			{Type: "reactions", Value: 4, Unit: "count"},
			{Type: "engagementRate", Value: 1.25, Unit: "percentage"},
		},
	}
}

func TestStatsRowPrintsMissingMetricsAsNA(t *testing.T) {
	cols := strings.Split(statsRow(samplePost()), "\t")
	header := strings.Split(statsHeader(), "\t")
	if len(cols) != len(header) {
		t.Fatalf("row has %d columns, header has %d", len(cols), len(header))
	}
	got := map[string]string{}
	for i, h := range header {
		got[h] = cols[i]
	}
	want := map[string]string{"impressions": "321", "reactions": "4", "engagementRate": "1.25", "comments": "n/a", "clicks": "n/a", "reach": "n/a"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if strings.Contains(got["text"], "\n") {
		t.Errorf("text column must be single-line, got %q", got["text"])
	}
}

func TestParseSentCount(t *testing.T) {
	cases := []struct {
		args    []string
		want    int
		wantErr bool
	}{
		{nil, 10, false},
		{[]string{"3"}, 3, false},
		{[]string{"50"}, 50, false},
		{[]string{"0"}, 0, true},
		{[]string{"51"}, 0, true},
		{[]string{"abc"}, 0, true},
	}
	for _, tc := range cases {
		got, err := parseSentCount(tc.args)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("parseSentCount(%v) = %d, %v; want %d, err=%v", tc.args, got, err, tc.want, tc.wantErr)
		}
	}
}
