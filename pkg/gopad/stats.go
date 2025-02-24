package gopad

type Stats struct {
	StartTime    int64 `json:"start_time"`
	NumDocuments int   `json:"num_documents"`
	DatabaseSize int   `json:"database_size"`
}
