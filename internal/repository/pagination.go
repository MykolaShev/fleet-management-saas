package repository

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// normalizePage clamps page/pageSize to sane bounds so callers can't pass
// e.g. page_size=100000 and blow up a query.
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}