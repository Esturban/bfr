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
	if err != nil || n < 1 {
		return 0, fmt.Errorf("--sent takes a number of 1 or more, got %q", args[0])
	}
	return n, nil
}

// statsHelp is what `bfr stats --help` prints.
const statsHelp = `Usage: bfr stats <post-id>
       bfr stats --sent [n]

  bfr stats <post-id>   metrics for one SENT post: impressions, reach, reactions,
                        comments, clicks, engagement rate. A metric Buffer does not
                        return prints n/a (clicks is not returned for LinkedIn).
  bfr stats --sent [n]  same columns for the n most recently sent posts (default 10,
                        no cap; pages through Buffer 50 at a time), newest first.

Read-only: never posts, schedules, updates or deletes.
`

// isHelpArg reports whether a stats argument asks for help rather than
// naming a post.
func isHelpArg(a string) bool {
	return a == "-h" || a == "--help" || a == "help"
}

// cmdStats prints post-level metrics for one post. Read-only.
func cmdStats(postID string) {
	if isHelpArg(postID) {
		fmt.Print(statsHelp)
		return
	}
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
	if len(args) > 0 && isHelpArg(args[0]) {
		fmt.Print(statsHelp)
		return
	}
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
