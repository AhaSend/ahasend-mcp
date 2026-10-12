package ahasendmcp

import (
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// List paging limits.
const (
	defaultListLimit = 20
	maxListLimit     = 100
)

// pageInput is the paging input every list tool takes.
type pageInput struct {
	Limit  int    `json:"limit,omitempty" jsonschema:"Number of items to return, from 1 to 100. Default 20."`
	Cursor string `json:"cursor,omitempty" jsonschema:"The next_cursor of the previous page, to read the page after it. Omit for the first page."`
}

// params checks the limit and returns the API's paging parameters.
func (p pageInput) params() (common.PaginationParams, error) {
	limit := p.Limit
	if limit == 0 {
		limit = defaultListLimit
	}
	if limit < 1 || limit > maxListLimit {
		return common.PaginationParams{}, invalidInput("limit must be from 1 to %d.", maxListLimit)
	}
	params := common.PaginationParams{Limit: ptr(int32(limit))}
	if p.Cursor != "" {
		params.After = ptr(p.Cursor)
	}
	return params, nil
}

// pageOutput is the paging part of every list tool's result.
type pageOutput struct {
	HasMore    bool   `json:"has_more" jsonschema:"True when more items follow this page."`
	NextCursor string `json:"next_cursor,omitempty" jsonschema:"Pass as cursor to read the next page. Missing on the last page."`
}

func pageOutputFrom(p common.PaginationInfo) pageOutput {
	out := pageOutput{HasMore: p.HasMore}
	if p.NextCursor != nil {
		out.NextCursor = *p.NextCursor
	}
	return out
}

// contactPageOutputFrom reads the paging of the contact and list endpoints.
func contactPageOutputFrom(p responses.ContactPagination) pageOutput {
	return pageOutputFrom(common.PaginationInfo{HasMore: p.HasMore, NextCursor: p.NextCursor})
}
