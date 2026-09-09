package search

type SearchBy string

const (
	SearchByFilename SearchBy = "filename"
	SearchByDate     SearchBy = "date"
	SearchByAuthor   SearchBy = "author"
	SearchByCreator  SearchBy = "creator"
	SearchByText     SearchBy = "text"
	SearchByType     SearchBy = "type"
)

type SearchQuery struct {
	By    SearchBy
	Value string
}
