package datatables

import (
	"strconv"
	"strings"

	"github.com/bonarizki/gin-datatables/v2/datatables/dto"
	"github.com/gin-gonic/gin"
)

// maxOrderClauses caps how many order[i] entries are read from the request,
// so a request with an excessive number of order clauses cannot force
// unbounded work.
const maxOrderClauses = 10

// maxColumns caps how many columns[i] entries are read from the request when
// looking for per-column search values. It is sized for wide tables rather
// than sort clauses, but still bounds the work a single request can force.
const maxColumns = 100

// ParseParams reads and normalizes query parameters used by the DataTables frontend.
// It extracts pagination, sorting, and search information into a standardized dto.Params struct.
//
// Supported DataTables parameters:
//   - draw: Draw counter for synchronization
//   - start: Record offset for pagination
//   - length: Number of records per page (bounded by opts.DefaultPageSize/MaxPageSize)
//   - search[value]: Global search value
//   - order[i][column]: Column to order by (numeric index into columns[], or a direct key)
//   - order[i][dir]: Order direction (asc/desc)
//   - order[i][name]: Column name as configured via columns.name; takes
//     priority over order[i][column] when present
//   - columns[i][data]: Column's data source, used to resolve a numeric
//     order[i][column] and to identify which column a per-column search
//     value belongs to
//   - columns[i][search][value]: Per-column search value
//
// Intentionally not supported:
//   - search[regex] / columns[i][search][regex]: treating the search value
//     as a regular expression is not implemented. Translating it to SQL is
//     not portable across database engines, and an untrusted regex is a
//     known denial-of-service vector (catastrophic backtracking).
//   - columns[i][searchable] / columns[i][orderable]: these are client-sent
//     hints and are never read. Trusting them would let a client widen its
//     own search/order surface; the searchable/orderable arguments passed
//     to OfReturn remain the only source of truth for what is allowed.
//
// The returned Length is always a positive number of rows unless
// opts.UnlimitedPageSize is true and the client explicitly requested -1;
// callers never need to special-case "no limit" themselves.
func ParseParams(c *gin.Context, opts Options) dto.Params {
	opts = opts.normalized()
	draw, err := strconv.ParseInt(c.DefaultQuery("draw", "1"), 10, 64)
	if err != nil {
		draw = 1
	}

	start, err := strconv.Atoi(c.DefaultQuery("start", "0"))
	if err != nil || start < 0 {
		start = 0
	}

	length := parseLength(c, opts)

	search := c.DefaultQuery("search[value]", "")

	return dto.Params{
		Draw:           draw,
		Start:          start,
		Length:         length,
		Search:         search,
		Orders:         parseOrders(c),
		ColumnSearches: parseColumnSearches(c),
	}
}

// parseLength resolves the page size, always returning either a positive
// row count or -1 when the caller has explicitly opted into unlimited pages.
func parseLength(c *gin.Context, opts Options) int {
	raw, exists := c.GetQuery("length")
	if !exists {
		return opts.DefaultPageSize
	}

	length, err := strconv.Atoi(raw)
	if err != nil {
		return opts.DefaultPageSize
	}

	if length == -1 {
		if opts.UnlimitedPageSize {
			return -1
		}
		return opts.DefaultPageSize
	}

	if length <= 0 {
		return opts.DefaultPageSize
	}

	if length > opts.MaxPageSize {
		return opts.MaxPageSize
	}

	return length
}

// parseOrders reads order[0], order[1], ... until the client stops sending
// them (or maxOrderClauses is reached), preserving shift-click multi-sort.
func parseOrders(c *gin.Context) []dto.Order {
	var orders []dto.Order

	for i := 0; i < maxOrderClauses; i++ {
		prefix := "order[" + strconv.Itoa(i) + "]"

		rawColumn, exists := c.GetQuery(prefix + "[column]")
		if !exists {
			break
		}

		name := c.Query(prefix + "[name]")
		col := resolveOrderColumn(c, rawColumn, name)
		if col == "" {
			continue
		}

		dir := strings.ToLower(c.DefaultQuery(prefix+"[dir]", "asc"))
		if dir != "asc" && dir != "desc" {
			dir = "asc"
		}

		orders = append(orders, dto.Order{Column: col, Dir: dir})
	}

	return orders
}

// resolveOrderColumn implements the DataTables order[i] contracts, in
// priority order:
//   - name: order[i][name], the column's configured name (columns.name),
//     when the client sends it
//   - Official: raw as a numeric index into columns[], whose name lives in
//     columns[i][data]
//   - Shorthand: raw as the orderable key sent directly
func resolveOrderColumn(c *gin.Context, raw string, name string) string {
	if name != "" {
		return name
	}

	if raw == "" {
		return ""
	}

	if _, err := strconv.Atoi(raw); err == nil {
		if colName := c.Query("columns[" + raw + "][data]"); colName != "" {
			return colName
		}
	}

	return raw
}

// parseColumnSearches reads columns[0][search][value], columns[1][search][value],
// ... until the client stops sending columns[i][data] (or maxColumns is
// reached). A column with no search value is skipped, not treated as the
// end of the list, since DataTables sends an entry for every column
// regardless of whether the user typed anything into it.
func parseColumnSearches(c *gin.Context) []dto.ColumnSearch {
	var searches []dto.ColumnSearch

	for i := 0; i < maxColumns; i++ {
		prefix := "columns[" + strconv.Itoa(i) + "]"

		col, exists := c.GetQuery(prefix + "[data]")
		if !exists {
			break
		}

		value := c.DefaultQuery(prefix+"[search][value]", "")
		if col == "" || value == "" {
			continue
		}

		searches = append(searches, dto.ColumnSearch{Column: col, Value: value})
	}

	return searches
}
