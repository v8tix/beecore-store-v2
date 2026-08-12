package response_test

import (
	"html"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/v8tix/beecore-store-v2/assets"
)

// staticRefPattern extracts literal /static/... paths from href=, src=, and
// inline url(...) attributes in template HTML, or url(...) references
// inside CSS/JS. Requires double-quoted href/src (misses srcset= and
// single-quoted attributes — neither pattern appears in this codebase
// today, but a future one would silently bypass this test). Only matches
// absolute /static/... paths — every template in this repo uses that
// convention consistently (no ../../static/... relative paths, unlike the
// beecore-admin-v2 sibling repo before its own fix for this same gap).
var staticRefPattern = regexp.MustCompile(`(?:href|src)="(/static/[^"?#]+)|url\(['"]?(/static/[^'")?#]+)`)

// TestReferencedStaticResourcesExist walks every template, then recurses one
// level into any referenced CSS/JS file, extracting every /static/... path
// along the way and confirming it exists in the embedded assets FS. This is
// the regression guard for beecore-7: a template or stylesheet referencing a
// resource that got deleted fails here, at test time, not as a broken image
// or missing stylesheet in production.
func TestReferencedStaticResourcesExist(t *testing.T) {
	seen := map[string]bool{}
	var toCheck []string

	templateFiles, err := fs.Glob(assets.EmbeddedFiles, "templates/*.html")
	if err != nil {
		t.Fatalf("glob base templates: %v", err)
	}
	pageFiles, err := fs.Glob(assets.EmbeddedFiles, "templates/pages/*.html")
	if err != nil {
		t.Fatalf("glob page templates: %v", err)
	}
	partialFiles, err := fs.Glob(assets.EmbeddedFiles, "templates/partials/*.html")
	if err != nil {
		t.Fatalf("glob partial templates: %v", err)
	}
	templateFiles = append(append(templateFiles, pageFiles...), partialFiles...)

	for _, tf := range templateFiles {
		toCheck = append(toCheck, extractStaticRefs(t, tf)...)
	}

	// One level of recursion: any referenced CSS/JS file may itself
	// reference fonts/images/source maps.
	var cssJSRefs []string
	for _, ref := range toCheck {
		if strings.HasSuffix(ref, ".css") || strings.HasSuffix(ref, ".js") {
			cssJSRefs = append(cssJSRefs, "static"+strings.TrimPrefix(ref, "/static"))
		}
	}
	for _, embedPath := range cssJSRefs {
		toCheck = append(toCheck, extractStaticRefs(t, embedPath)...)
	}

	for _, ref := range toCheck {
		if seen[ref] {
			continue
		}
		seen[ref] = true

		embedPath := "static" + strings.TrimPrefix(ref, "/static")
		if _, err := fs.Stat(assets.EmbeddedFiles, embedPath); err != nil {
			t.Errorf("referenced resource missing from embedded assets: %s", ref)
		}
	}

	if len(seen) == 0 {
		t.Fatal("extracted zero static references — pattern or glob is broken, this test would silently pass on anything")
	}
}

func extractStaticRefs(t *testing.T, embedPath string) []string {
	t.Helper()

	content, err := fs.ReadFile(assets.EmbeddedFiles, embedPath)
	if err != nil {
		t.Fatalf("read %s: %v", embedPath, err)
	}

	var refs []string
	for _, m := range staticRefPattern.FindAllStringSubmatch(string(content), -1) {
		for _, group := range m[1:] {
			if group != "" && strings.HasPrefix(group, "/static/") {
				// Templates may HTML-entity-encode characters like & in
				// literal attribute values (e.g. b&amp;w/ -> b&w/ on disk).
				refs = append(refs, html.UnescapeString(group))
			}
		}
	}
	return refs
}
