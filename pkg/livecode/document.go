package livecode

type PersistedDocument struct {
	Text     string  `json:"text"`
	Language *string `json:"language"`
}
