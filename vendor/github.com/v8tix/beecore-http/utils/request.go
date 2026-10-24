package utils

import (
	"errors"
	"net/http"
)

// GetQueryParam returns the value of key from r's query string, or "" if absent.
func GetQueryParam(r *http.Request, key string) string {
	query := r.URL.Query()

	return query.Get(key)
}

// ReadParam returns the value of key from r's query string, or an error if it's missing or empty.
func ReadParam(r *http.Request, key string) (string, error) {
	param := r.URL.Query().Get(key)
	if param == "" {
		return "", errors.New("invalid id parameter")
	}

	return param, nil
}
