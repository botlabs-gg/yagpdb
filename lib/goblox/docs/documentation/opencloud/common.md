# Common
These are some commonly used structures for the Opencloud API.

## Methods
### `Pointer`
This is a utility method to easily create pointers.
```go
func Pointer(v any) *any
```

## Query Parameters
### `Options`
```go
type Options struct {
	MaxPageSize *int    `url:"maxPageSize,omitempty"`
	PageToken   *string `url:"pageToken,omitempty"`
}
```
### `OptionsWithFilter`
```go
type OptionsWithFilter struct {
	Options
	Filter *string `url:"filter,omitempty"`
}
```

## Responses
### `Operation`
```go
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
```