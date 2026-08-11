package utils

import (
	"net/url"
	"strconv"
	"strings"
)

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

func HasPreviousPage(currentPage int64) (int64, bool) {
	previousPage := currentPage - 1
	if previousPage >= 1 {
		return previousPage, true
	}
	return 0, false
}

func HasNextPage(totalPages, currentPage int64) (int64, bool) {
	nextPage := currentPage + 1
	if nextPage <= totalPages {
		return nextPage, true
	}
	return 0, false
}

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
			if strings.Contains(rel, `rel="next"`) {
				links["next"] = page
			} else if strings.Contains(rel, `rel="prev"`) {
				links["prev"] = page
			} else if strings.Contains(rel, `rel="self"`) {
				links["self"] = page
			} else if strings.Contains(rel, `rel="first"`) {
				links["first"] = page
			} else if strings.Contains(rel, `rel="last"`) {
				links["last"] = page
			}
		}
	}

	return links
}
