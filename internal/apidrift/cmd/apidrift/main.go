// Command apidrift checks the fields this provider decodes against Anthropic's
// API reference and writes a markdown report.
//
// Exit status: 0 when there is nothing to act on, 3 when the report has
// findings or release notes to read, and 1 when the check itself could not run,
// for example because a manifest page no longer exists. A broken check exits 1
// rather than reporting "nothing to act on", because silence from a check that
// did not run is how a real deprecation goes unnoticed.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/uttej-badwane/terraform-provider-anthropic-enterprise/internal/apidrift"
)

const (
	exitClean    = 0
	exitBroken   = 1
	exitFindings = 3
	userAgent    = "terraform-provider-anthropic-enterprise api-drift (+https://github.com/uttej-badwane/terraform-provider-anthropic-enterprise)"
)

func main() {
	repo := flag.String("repo", ".", "repository root")
	out := flag.String("report", "", "write the markdown report here (default stdout)")
	days := flag.Int("days", 8, "include release notes from this many days back")
	base := flag.String("base", "https://platform.claude.com/docs/en", "documentation root")
	flag.Parse()
	os.Exit(run(*repo, *out, *days, *base))
}

func run(repo, out string, days int, base string) int {
	fail := func(errs ...error) int {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "apidrift:", e)
		}
		return exitBroken
	}

	m, err := apidrift.LoadManifest()
	if err != nil {
		return fail(err)
	}
	types, err := apidrift.LoadTypes(filepath.Join(repo, "internal", "client"))
	if err != nil {
		return fail(err)
	}
	if errs := apidrift.Validate(m, types); len(errs) > 0 {
		return fail(errs...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 30 * time.Second}

	deps := map[string][]apidrift.Deprecation{}
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		errs  []error
		slots = make(chan struct{}, 6)
	)
	for _, p := range m.Pages {
		wg.Add(1)
		go func(page string) {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			body, err := fetch(ctx, client, base+"/api/"+page+".md")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", page, err))
				return
			}
			deps[page] = apidrift.ParseDeprecations(page, body)
		}(p.Page)
	}
	wg.Wait()
	if len(errs) > 0 {
		return fail(errs...)
	}

	now := time.Now().UTC()
	since := now.AddDate(0, 0, -days).Truncate(24 * time.Hour)
	notesMD, err := fetch(ctx, client, base+"/release-notes/api.md")
	if err != nil {
		return fail(fmt.Errorf("release notes: %w", err))
	}
	notes := apidrift.ParseReleaseNotes(notesMD, since)

	result := apidrift.Check(m, types, deps)
	report := apidrift.Report(result, notes, since, now)
	if out == "" {
		fmt.Print(report)
	} else if err := os.WriteFile(out, []byte(report), 0o600); err != nil {
		return fail(err)
	}

	fmt.Fprintf(os.Stderr, "apidrift: %d page(s), %d finding(s), %d fallback(s), %d release note(s)\n",
		len(m.Pages), len(result.Findings), len(result.Fallbacks), len(notes))
	if len(result.Findings) > 0 || len(notes) > 0 {
		return exitFindings
	}
	return exitClean
}

// fetch retries once, so a single dropped connection does not fail the run.
func fetch(ctx context.Context, c *http.Client, url string) (string, error) {
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "text/markdown")
		resp, err := c.Do(req)
		if err != nil {
			last = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if err != nil {
			last = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			// A 404 means the page moved or the endpoint was removed, which
			// is itself something to know about, so it is not retried.
			return "", fmt.Errorf("GET %s: %s", url, resp.Status)
		}
		return string(body), nil
	}
	return "", errors.Join(fmt.Errorf("GET %s failed twice", url), last)
}
