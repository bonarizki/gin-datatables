package datatables

import (
	"strconv"
	"strings"

	"github.com/bonarizki-dat/Datatables-Gin/v2/datatables/dto"
	"github.com/gin-gonic/gin"
)

// maxOrderClauses caps how many order[i] entries are read from the request,
// so a request with an excessive number of order clauses cannot force
// unbounded work.
const maxOrderClauses = 10

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
//
// The returned Length is always a positive number of rows unless
// opts.UnlimitedPageSize is true and the client explicitly requested -1;
// callers never need to special-case "no limit" themselves.
func ParseParams(c *gin.Context, opts Options) dto.Params {
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
		Draw:   draw,
		Start:  start,
		Length: length,
		Search: search,
		Orders: parseOrders(c),
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

		col := resolveOrderColumn(c, rawColumn)
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

// resolveOrderColumn implements both DataTables order[i][column] contracts:
//   - Official: a numeric index into columns[], whose name lives in columns[i][data]
//   - Shorthand: the orderable key sent directly
func resolveOrderColumn(c *gin.Context, raw string) string {
	if raw == "" {
		return ""
	}

	if _, err := strconv.Atoi(raw); err == nil {
		if name := c.Query("columns[" + raw + "][data]"); name != "" {
			return name
		}
	}

	return raw
}
