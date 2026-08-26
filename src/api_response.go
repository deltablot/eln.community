package main

type ResponseError struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description"`
}

type Pagination struct {
	Page       int `json:"page,omitempty"`
	PageSize   int `json:"page_size,omitempty"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages"`
}

type ResponseMeta struct {
	Pagination Pagination    `json:"pagination"`
	Errors     ResponseError `json:"errors"`
}

type RecordResponse struct {
	Data []Record     `json:"data"`
	Meta ResponseMeta `json:"meta"`
}
