package main

type ResponseError struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description"`
}

type Pagination struct {
	Page       int `json:"page,omitempty"`
	Limit      int `json:"limit"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages"`
}

type ResponseMeta struct {
	Pagination Pagination    `json:"pagination"`
	Error      ResponseError `json:"errors"`
}

type RecordResponse struct {
	Data          []Record          `json:"data"`
	Organizations []RorOrganization `json:"organizations"`
	Meta          ResponseMeta      `json:"meta"`
}
