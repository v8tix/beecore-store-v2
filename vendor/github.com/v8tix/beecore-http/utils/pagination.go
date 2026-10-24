package utils

import (
	"net/url"
	"strconv"
	"strings"
)

// NumberOfPages returns how many pages of perPage items are needed to hold numberOfElement items.
func NumberOfPages(numberOfElement, perPage int64) int64 {
	var totalPages int64

	pages := numberOfElement / perPage

	extraPages := numberOfElement % perPage
	if extraPages > 0 {
		totalPages = pages + 1
	} else {
		totalPages = pages
	}

	return totalPages
}

// HasPreviousPage reports whether currentPage has a previous page, returning its number.
func HasPreviousPage(currentPage int64) (int64, bool) {
	previousPage := currentPage - 1
	if previousPage >= 1 {
		return previousPage, true
	}

	return 0, false
}

// HasNextPage reports whether currentPage has a next page (bounded by totalPages), returning its number.
func HasNextPage(totalPages, currentPage int64) (int64, bool) {
	nextPage := currentPage + 1
	if nextPage <= totalPages {
		return nextPage, true
	}

	return 0, false
}

// ParseLinkHeader parses an RFC 8288 Link header into a map of relation name ("next",
// "prev", "self", "first", "last") to page number.
func ParseLinkHeader(linkHeader string) map[string]int {
	links := map[string]int{}

	parts := strings.Split(linkHeader, ",")
	for _, part := range parts {
		section := strings.Split(strings.TrimSpace(part), ";")
		if len(section) < 2 {
			continue
		}

		link := strings.Trim(section[0], "<>")

		urlObj, err := url.Parse(link)
		if err != nil {
			continue
		}

		pageParam := urlObj.Query().Get("page")

		page, err := strconv.Atoi(pageParam)
		if err != nil {
			continue
		}

		for _, rel := range section[1:] {
			if key := linkRelation(rel); key != "" {
				links[key] = page
			}
		}
	}

	return links
}

// linkRelation extracts the relation name ("next", "prev", "self", "first", "last") from
// one semicolon-separated segment of a Link header entry, or "" if it names none of them.
func linkRelation(rel string) string {
	switch {
	case strings.Contains(rel, `rel="next"`):
		return "next"
	case strings.Contains(rel, `rel="prev"`):
		return "prev"
	case strings.Contains(rel, `rel="self"`):
		return "self"
	case strings.Contains(rel, `rel="first"`):
		return "first"
	case strings.Contains(rel, `rel="last"`):
		return "last"
	default:
		return ""
	}
}
