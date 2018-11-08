package patreonapi

type Meta struct {
	Pagination *Pagination `json:"pagination"`
}

type Pagination struct {
	Cursors Cursors `json:"cursors"`
	Total   int     `json:"total"`
}

type Cursors struct {
	Next string `json:"next"`
}
