package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Esturban/bfr/bufferclient"
)

// root is the current working directory -- where a caller runs `bfr` from,
// not where the binary happens to be installed. .env and the channel cache
// default here, same convention `git` and most CLIs use for repo-local
// config, and it works the same way whether bfr is built to ./bfr, `go
// install`ed onto $PATH, or placed anywhere else.
func root() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

// loadEnvFile reads simple KEY=VALUE lines from path into the process
// environment, without overriding anything already set. Missing file is not
// an error -- env vars are the primary source, this is a convenience.
func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if _, already := os.LookupEnv(key); !already {
			os.Setenv(key, val)
		}
	}
}

// ensureEnv replicates the bash tool's env setup exactly: if BUFFER_API_KEY
// isn't already exported, source the WHOLE .env file (which is also where
// BUFFER_DRIVE_ACCOUNT and anything else downstream verbs need lives) --
// same gate, same side effect. Called once at startup, before any verb
// dispatches, so every later os.Getenv in this program sees what .env set,
// not just BUFFER_API_KEY.
func ensureEnv() {
	if os.Getenv("BUFFER_API_KEY") != "" {
		return
	}
	loadEnvFile(filepath.Join(root(), ".env"))
}

// apiToken resolves BUFFER_API_KEY from the environment. ensureEnv must have
// already run. Never a flag, never echoed, never written anywhere.
func apiToken() (string, error) {
	if tok := os.Getenv("BUFFER_API_KEY"); tok != "" {
		return tok, nil
	}
	return "", fmt.Errorf("BUFFER_API_KEY not set. Export it, or put it in this repo's .env (gitignored)")
}

// cachePath is the channel cache file location: BUFFER_CACHE_FILE overrides
// it, otherwise it defaults next to the repo root, same file the original
// wrote to (.bfr-channels.json).
func cachePath() string {
	if p := os.Getenv("BUFFER_CACHE_FILE"); p != "" {
		return p
	}
	return filepath.Join(root(), ".bfr-channels.json")
}

type cacheFile struct {
	OrganizationID string                 `json:"organizationId"`
	Channels       []bufferclient.Channel `json:"channels"`
	CachedAt       string                 `json:"cachedAt"`
}

func readCache() (*cacheFile, error) {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return nil, fmt.Errorf("no channel cache. Run 'bfr channels' first")
	}
	var c cacheFile
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("channel cache at %s is not valid JSON: %w", cachePath(), err)
	}
	return &c, nil
}

func writeCache(orgID string, channels []bufferclient.Channel) error {
	c := cacheFile{OrganizationID: orgID, Channels: channels, CachedAt: time.Now().UTC().Format(time.RFC3339)}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath(), data, 0o644)
}

// resolveChannel looks up a channel by id or case-insensitive name in the
// cache. Same contract as the bash tool: BLOCKED, not a fresh API call, if
// the cache is missing or the channel isn't in it.
func resolveChannel(arg string) (string, error) {
	c, err := readCache()
	if err != nil {
		return "", err
	}
	for _, ch := range c.Channels {
		if ch.ID == arg || strings.EqualFold(ch.Name, arg) {
			return ch.ID, nil
		}
	}
	return "", fmt.Errorf("channel '%s' not in cache. Run 'bfr channels' to refresh", arg)
}

// channelServiceByID looks up a channel's service (e.g. "linkedin",
// "twitter") from the cache by id. Same cache resolveChannel already reads
// -- no fresh API call. Used to gate --first-comment to LinkedIn channels
// only (CMO-2596): the flag must error, not silently no-op, on any other
// service.
func channelServiceByID(channelID string) (string, error) {
	c, err := readCache()
	if err != nil {
		return "", err
	}
	for _, ch := range c.Channels {
		if ch.ID == channelID {
			return ch.Service, nil
		}
	}
	return "", fmt.Errorf("channel id '%s' not in cache. Run 'bfr channels' to refresh", channelID)
}

// orgID returns the cached organizationId if a cache file exists, otherwise
// resolves it live from the API (without caching it standalone -- org is
// only persisted as part of a full 'bfr channels' write, same as bash).
func orgID(client *bufferclient.Client) (string, error) {
	if c, err := readCache(); err == nil {
		return c.OrganizationID, nil
	}
	id, resp, err := client.OrganizationID()
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("could not resolve organizationId. Response:\n%s", resp)
	}
	return id, nil
}

// readBody reads a file's text body. The bash tool captured this via
// text=$(cat "$file") -- command substitution, which strips ALL trailing
// newlines. Buffer's API does not itself trim trailing newlines from text
// (verified live: a file with one trailing \n, sent as-is, comes back with
// one extra \n appended by Buffer on top of whatever was sent). Matching
// TrimRight here, not a raw read, is what makes the two tools send byte-
// identical text -- a raw read would silently double the trailing newline.
func readBody(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", path)
	}
	return strings.TrimRight(string(data), "\n"), nil
}

// truncate slices s to at most n runes, matching jq's .[0:n] slicing
// (codepoint-based, not byte-based) plus an appended "..." when clipped.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}
