//go:build e2e

// Session is a trimmed port of the Session type in inside-agent-web-ui's
// internal/eval/infrastructure/devtools package, cut down to just what
// this test needs (Open, Call, Close) -- no multi-tab driver, no
// accessibility-tree snapshot/query engine, no audits. Same underlying
// pattern: spawn the pinned, official chrome-devtools-mcp server as an
// isolated child process and speak MCP to it over stdio. Ported verbatim
// from beecore-admin-v2/e2e/mcp_session.go — nothing here is
// admin-v2-specific except the client name/version below.
package e2e

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpPackage is the exact, pinned chrome-devtools-mcp release this test
// runs against -- a new release of the package can't silently change this
// test's behavior. Matches the version beecore-admin-v2/e2e pins.
const mcpPackage = "chrome-devtools-mcp@1.10.1"

// mcpPrivacyArgs keep the server from sending usage statistics, querying
// CrUX, or touching a real Chrome profile -- same four flags
// inside-agent-web-ui's config.CheckMCP requires, for the same reason.
var mcpPrivacyArgs = []string{"--isolated", "--no-usage-statistics", "--no-performance-crux"}

const (
	startTimeout  = 3 * time.Minute // first run may download the npm package
	callTimeout   = time.Minute
	terminateWait = 5 * time.Second
)

// clientVersion is what this test calls itself in the MCP handshake.
const clientVersion = "beecore-store-v2-e2e/0.1.0"

// mcpSession is a connection to one chrome-devtools-mcp server, and so to
// one Chrome instance.
type mcpSession struct {
	cs   *mcp.ClientSession
	kill context.CancelFunc

	dead      atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

// openMCPSession starts chrome-devtools-mcp (headless unless headed is
// true) and completes the MCP handshake.
func openMCPSession(ctx context.Context, headed bool) (*mcpSession, error) {
	args := slices.Clone(mcpPrivacyArgs)
	if !headed {
		args = append(args, "--headless")
	}

	procCtx, kill := context.WithCancel(context.WithoutCancel(ctx))
	cmd := buildMCPCommand(procCtx, args)

	startCtx, cancel := context.WithTimeout(ctx, startTimeout)
	defer cancel()
	giveUp := context.AfterFunc(startCtx, kill)

	client := mcp.NewClient(&mcp.Implementation{Name: "beecore-store-v2-e2e", Version: clientVersion}, nil)
	cs, err := client.Connect(startCtx, &mcp.CommandTransport{Command: cmd, TerminateDuration: terminateWait}, nil)
	if !giveUp() && err == nil { // the start timeout fired just as it answered
		err = startCtx.Err()
		if cs != nil {
			_ = cs.Close()
		}
	}
	if err != nil {
		kill()
		return nil, fmt.Errorf("starting chrome-devtools-mcp: %w", err)
	}

	return &mcpSession{cs: cs, kill: kill}, nil
}

// buildMCPCommand builds the process that runs chrome-devtools-mcp, pinned
// and isolated -- see proc_unix.go's isolate/killTree.
func buildMCPCommand(ctx context.Context, args []string) *exec.Cmd {
	argv := append([]string{"npx", "-y", mcpPackage}, args...)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Env = childEnv(os.Environ())
	cmd.Dir = os.TempDir()
	cmd.Cancel = func() error { killTree(cmd); return nil }
	isolate(cmd)
	return cmd
}

// childEnvNames/childEnvPrefixes/secretName/childEnv mirror
// inside-agent-web-ui's ChildEnv: the MCP server only gets what it needs
// to find Node and Chrome and write temp files -- nothing that looks like
// a credential, even though this test's own E2E_EMAIL/E2E_PASSWORD never
// need to reach a process that only drives a browser.
var childEnvNames = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true,
	"TMPDIR": true, "TMP": true, "TEMP": true, "TERM": true, "TZ": true,
	"LANG": true, "LANGUAGE": true,
	"DISPLAY": true, "WAYLAND_DISPLAY": true, "XAUTHORITY": true, "DBUS_SESSION_BUS_ADDRESS": true,
	"SSL_CERT_FILE": true, "SSL_CERT_DIR": true,
	"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true, "ALL_PROXY": true,
}

var childEnvPrefixes = []string{"LC_", "XDG_", "NODE_", "NPM_", "NVM_", "FNM_", "VOLTA_", "ASDF_", "CHROME_", "PUPPETEER_"}

var secretName = regexp.MustCompile(`(?i)(password|passwd|secret|token|credential|api[_-]?key|private[_-]?key|cookie)`)

func childEnv(environ []string) []string {
	var out []string
	for _, kv := range environ {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || name == "" || secretName.MatchString(name) {
			continue
		}
		upper := strings.ToUpper(name)
		if childEnvNames[upper] || slices.ContainsFunc(childEnvPrefixes, func(p string) bool { return strings.HasPrefix(upper, p) }) {
			out = append(out, kv)
		}
	}
	return out
}

// call runs one tool and returns its text content.
func (s *mcpSession) call(ctx context.Context, tool string, args map[string]any) (string, error) {
	if s.dead.Load() {
		return "", fmt.Errorf("session is closed or was lost (calling %s)", tool)
	}

	callCtx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	res, err := s.cs.CallTool(callCtx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		if isGone(err) {
			s.dead.Store(true)
		}
		return "", fmt.Errorf("calling %s: %w", tool, err)
	}

	var texts []string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			texts = append(texts, tc.Text)
		}
	}
	text := strings.Join(texts, "\n")

	if res.IsError {
		return "", fmt.Errorf("%s: %s", tool, strings.TrimSpace(text))
	}

	return text, nil
}

func isGone(err error) bool {
	if errors.Is(err, mcp.ErrConnectionClosed) || errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, os.ErrProcessDone) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "file already closed") || strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection closed")
}

// close ends the session: the server is asked to stop by closing its
// input, and if ctx ends first, it and the browser it started are killed.
// Safe to call more than once.
func (s *mcpSession) close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		s.dead.Store(true)
		done := make(chan error, 1)
		go func() { done <- s.cs.Close() }()
		select {
		case err := <-done:
			s.closeErr = ignoreExit(err)
		case <-ctx.Done():
			s.kill()
			s.closeErr = fmt.Errorf("closing chrome-devtools-mcp: %w", ctx.Err())
		}
	})
	return s.closeErr
}

func ignoreExit(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}
	return err
}
