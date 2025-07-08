package opencloud

// --- Response Structures

type OperationResponse any

type OperationError any

type OperationMetadata any

type Operation struct {
	Path     string            `json:"path"`
	Done     bool              `json:"done"`
	Error    *OperationError   `json:"error,omitempty"`
	Response OperationResponse `json:"response"`
	Metadata OperationMetadata `json:"metadata"`
}

// --- Query Options

type Options struct {
	MaxPageSize *int    `url:"maxPageSize,omitempty"`
	PageToken   *string `url:"pageToken,omitempty"`
}

type OptionsWithFilter struct {
	Options
	Filter *string `url:"filter,omitempty"`
}

/// --- Pointer

func Pointer[T any](v T) *T {
	return &v
}
