//go:build e2e

// Package e2e holds browser-driven end-to-end tests that run against a
// live, already-running beecore-store-v2 instance (and whatever it talks
// to downstream — beecore-stores, beecore-baskets, beecore-ordering,
// beecore-payments, Redis, etc.). Ported from beecore-admin-v2/e2e
// (TestTaxonomyComboBox_FiltersAndSelectsLeaf) — same harness verbatim
// (mcp_session.go, proc_unix.go/proc_other.go, helpers_test.go), since
// nothing in it is admin-v2-specific. They drive the browser through the
// official chrome-devtools-mcp server (pinned, see mcp_session.go),
// speaking MCP via the official Go SDK
// (github.com/modelcontextprotocol/go-sdk) -- the same approach and the
// same server inside-agent-web-ui's own eval framework
// (internal/eval/infrastructure/devtools) uses, trimmed down here to just
// what this package needs (no multi-tab driver, no accessibility-tree
// snapshot/query engine, no audits).
//
// They are not part of `make test`/`make audit`: no CI environment here
// has Node, a browser, or a live deployment to point at, and nothing in
// this package is reachable without the "e2e" build tag.
//
// Requires Node/npx on PATH (chrome-devtools-mcp is fetched via npx on
// first run) and a locally installed Chrome.
//
// Run manually against a port-forwarded local/k3d deployment:
//
//	E2E_BASE_URL=http://localhost:4445 \
//	E2E_EMAIL=buyer@example.com \
//	E2E_PASSWORD=... \
//	go test -tags e2e ./e2e/...
//
// Set E2E_HEADED=1 to watch Chrome run instead of headless.
//
// Every test skips itself (not fails) when E2E_EMAIL or E2E_PASSWORD
// isn't set, so an accidental `go test -tags e2e ./...` without a live
// target configured doesn't break anything.
package e2e
