//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// config reads E2E_BASE_URL/E2E_EMAIL/E2E_PASSWORD, skipping the calling
// test when a credential isn't set. Ported from beecore-admin-v2/e2e's
// config, with the default base URL pointed at this service's own
// port-forward (4445, not admin-v2's 4444).
func config(t *testing.T) (baseURL, email, password string) {
	t.Helper()

	email = os.Getenv("E2E_EMAIL")
	password = os.Getenv("E2E_PASSWORD")
	if email == "" || password == "" {
		t.Skip("E2E_EMAIL/E2E_PASSWORD not set — skipping live e2e test")
	}

	baseURL = os.Getenv("E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:4445"
	}

	return baseURL, email, password
}

// newSession starts chrome-devtools-mcp (headless unless E2E_HEADED is
// set) and returns a session bound to a context with a 2-minute timeout,
// plus a cleanup func the caller must defer. Ported verbatim from
// beecore-admin-v2/e2e's newSession.
func newSession(t *testing.T) (*mcpSession, context.Context) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	s, err := openMCPSession(ctx, os.Getenv("E2E_HEADED") != "")
	if err != nil {
		t.Fatalf("starting chrome-devtools-mcp: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer closeCancel()
		_ = s.close(closeCtx)
	})

	return s, ctx
}

// openPage opens url in a new tab and returns its page ID. Ported
// verbatim from beecore-admin-v2/e2e's openPage.
func openPage(ctx context.Context, s *mcpSession, url string) (int, error) {
	text, err := s.call(ctx, "new_page", map[string]any{"url": url})
	if err != nil {
		return 0, err
	}
	id, ok := selectedPageID(text)
	if !ok {
		return 0, fmt.Errorf("new_page reported no selected page: %q", shorten(text, 200))
	}
	return id, nil
}

// selectedPageID finds the "N: Title (url) [selected]" line new_page and
// list_pages print and returns its leading page number. Mirrors the line
// format inside-agent-web-ui's devtools.ParsePages reads. Ported verbatim
// from beecore-admin-v2/e2e's selectedPageID.
func selectedPageID(text string) (int, bool) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r ")
		if !strings.Contains(line, "[selected]") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			if id, err := strconv.Atoi(line[:idx]); err == nil {
				return id, true
			}
		}
	}
	return 0, false
}

// eval runs function (a complete, no-argument JS arrow function as
// source, e.g. "() => document.title") in page's context and decodes the
// JSON it returns into out. waitForStableDom should be true after an
// action that triggers real navigation (e.g. a form submit), false for a
// same-page DOM read/write that shouldn't wait out a page that never goes
// idle. Ported verbatim from beecore-admin-v2/e2e's eval — same-document
// only; it cannot reach into a cross-origin iframe (e.g. PayPhone's own
// card-entry widget), which is why checkout_payphone_test.go uses
// snapshot/click/fill below for that part of the flow instead.
func eval(ctx context.Context, s *mcpSession, pageID int, function string, waitForStableDom bool, out any) error {
	text, err := s.call(ctx, "evaluate_script", map[string]any{
		"pageId":           pageID,
		"function":         function,
		"waitForStableDom": waitForStableDom,
	})
	if err != nil {
		return err
	}

	raw, err := parseScriptResult(text)
	if err != nil {
		return fmt.Errorf("reading evaluate_script's result: %w (raw: %q)", err, shorten(text, 200))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// parseScriptResult extracts the JSON evaluate_script prints between a
// "```json" line and the last closing "```" in its response. Mirrors
// inside-agent-web-ui's devtools.ParseScriptResult. Ported verbatim from
// beecore-admin-v2/e2e's parseScriptResult.
func parseScriptResult(text string) ([]byte, error) {
	const open, close = "```json", "```"
	start := strings.Index(text, open)
	end := strings.LastIndex(text, close)
	if start < 0 || end < start+len(open) {
		return nil, fmt.Errorf("evaluate_script printed no result")
	}
	if body := strings.TrimSpace(text[start+len(open) : end]); body != "" {
		return []byte(body), nil
	}
	return []byte("null"), nil
}

// waitUntil polls check every 200ms until it returns true or timeout
// elapses, returning the last error check reported (nil if it eventually
// succeeded). Ported verbatim from beecore-admin-v2/e2e's waitUntil.
func waitUntil(ctx context.Context, timeout time.Duration, check func() (bool, error)) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		ok, err := check()
		if err != nil {
			lastErr = err
		} else if ok {
			return nil
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return fmt.Errorf("timed out after %s: %w", timeout, lastErr)
			}
			return fmt.Errorf("timed out after %s waiting for condition", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// --- New below: not part of beecore-admin-v2/e2e's helpers. ---
//
// The taxonomy combobox test only ever reads/writes same-document DOM
// state via eval. checkout_payphone_test.go additionally has to drive
// PayPhone's own embedded payment widget, which chrome-devtools-mcp's own
// docs don't guarantee evaluate_script can reach (it runs in one page's
// JS context; a same-origin-policy-isolated widget frame is a different
// one). take_snapshot's accessibility tree and the click/fill tools
// operate at the CDP/Input-event level instead, the same layer real user
// input crosses frame boundaries at, so they're used for that part of the
// flow. Both helpers are intentionally parsed the same defensive way
// selectedPageID above reads new_page's text: by scanning the line-based
// text take_snapshot returns, not by assuming a specific machine-readable
// format.

// uidLine matches one take_snapshot line's leading "uid=<id>" token,
// capturing the id. take_snapshot's own doc only confirms the token's
// name (uid=...), not a fixed surrounding grammar, so this intentionally
// doesn't try to parse role/name structure beyond that.
var uidLine = regexp.MustCompile(`uid=(\S+)`)

// snapshot takes a fresh accessibility-tree snapshot of pageID. Callers
// must take a new one after any action that changes the DOM (e.g.
// selecting a shipping address, which mounts PayPhone's widget) — a uid
// from a stale snapshot may point at a removed or replaced element.
func snapshot(ctx context.Context, s *mcpSession, pageID int) (string, error) {
	return s.call(ctx, "take_snapshot", map[string]any{"pageId": pageID})
}

// findUID returns the uid of the first line in a take_snapshot result
// that contains every string in contains (case-insensitive) — e.g.
// findUID(snap, "textbox", "número de tarjeta") to find the card-number
// field by its role and placeholder/accessible name together, which is
// more specific than either alone.
func findUID(snapshotText string, contains ...string) (uid string, ok bool) {
	for _, line := range strings.Split(snapshotText, "\n") {
		lower := strings.ToLower(line)
		matched := true
		for _, want := range contains {
			if !strings.Contains(lower, strings.ToLower(want)) {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		if m := uidLine.FindStringSubmatch(line); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// click dispatches a real click at uid's element via CDP Input events —
// unlike eval's JS .click(), this crosses into a cross-origin iframe the
// same way a real user's click would.
func click(ctx context.Context, s *mcpSession, pageID int, uid string) error {
	_, err := s.call(ctx, "click", map[string]any{"pageId": pageID, "uid": uid})
	return err
}

// fill types value into uid's element via CDP Input events — see click's
// doc comment for why this (not eval) is used for PayPhone's widget
// fields.
func fill(ctx context.Context, s *mcpSession, pageID int, uid, value string) error {
	_, err := s.call(ctx, "fill", map[string]any{"pageId": pageID, "uid": uid, "value": value})
	return err
}
