package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Esturban/bfr/bufferclient"
)

const (
	defaultSentCount = 10
	notReturned      = "n/a"
)

// statsColumns is the fixed metric order for every stats output. clicks is
// listed because the log wants it, but Buffer does not return it for
// LinkedIn posts, so it prints n/a rather than a made-up zero.
var statsColumns = []string{"impressions", "reach", "reactions", "comments", "clicks", "engagementRate"}

// formatMetric renders one metric, or n/a when Buffer did not return it.
func formatMetric(s bufferclient.PostStats, metricType string) string {
	v, ok := s.Value(metricType)
	if !ok {
		return notReturned
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// statsRow is one tab-separated line: id, sentAt, then statsColumns, text.
func statsRow(s bufferclient.PostStats) string {
	cols := []string{s.ID, s.SentAt}
	for _, m := range statsColumns {
		cols = append(cols, formatMetric(s, m))
	}
	cols = append(cols, truncate(strings.ReplaceAll(s.Text, "\n", " "), 50))
	return strings.Join(cols, "\t")
}

func statsHeader() string {
	return "id\tsentAt\t" + strings.Join(statsColumns, "\t") + "\ttext"
}

// parseSentCount reads the optional n after --sent. Empty means the default.
func parseSentCount(args []string) (int, error) {
	if len(args) == 0 {
		return defaultSentCount, nil
	}
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 || n > bufferclient.MaxSentStats {
		return 0, fmt.Errorf("--sent takes a number from 1 to %d, got %q", bufferclient.MaxSentStats, args[0])
	}
	return n, nil
}

// cmdStats prints post-level metrics for one post. Read-only.
func cmdStats(postID string) {
	if strings.TrimSpace(postID) == "" {
		blocked("post id is required")
	}
	c := newClient()
	s, resp, err := c.GetStats(postID)
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	fmt.Println(statsHeader())
	fmt.Println(statsRow(*s))
	if s.Status != "sent" {
		fmt.Printf("note: post status is %q, metrics exist only for sent posts\n", s.Status)
	}
	if s.MetricsUpdatedAt != "" {
		fmt.Printf("metricsUpdatedAt: %s\n", s.MetricsUpdatedAt)
	}
}

// cmdStatsSent prints metrics for the n most recently sent posts. Read-only.
func cmdStatsSent(args []string) {
	n, err := parseSentCount(args)
	if err != nil {
		blocked("%s", err)
	}
	c := newClient()
	org, err := orgID(c)
	if err != nil {
		blocked("%s", err)
	}
	items, resp, err := c.ListSent(org, n)
	if err != nil {
		blockedResponse(err.Error(), resp)
	}
	fmt.Println(statsHeader())
	for _, s := range items {
		fmt.Println(statsRow(s))
	}
}
