package main

import (
	"strconv"
	"net/http"
)

const (
	defaultPaginationLimit = 20
	maxPaginationLimit     = 100
	defaultPaginationOffset = 0
)

func parsePagination(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > maxPaginationLimit {
		limit = defaultPaginationLimit
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = defaultPaginationOffset
	}
    return limit, offset
}
