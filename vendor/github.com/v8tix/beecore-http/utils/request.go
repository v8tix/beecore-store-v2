package utils

import (
	"errors"
	"net/http"
)

func GetQueryParam(r *http.Request, key string) string {
	query := r.URL.Query()
	return query.Get(key)
}

func ReadParam(r *http.Request, key string) (string, error) {
	param := r.URL.Query().Get(key)
	if param == "" {
		return "", errors.New("invalid id parameter")
	}
	return param, nil
}
