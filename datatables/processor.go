package datatables

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"github.com/bonarizki/gin-datatables/v2/datatables/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

// OfReturn executes the core DataTables server-side logic.
// It supports searching, ordering, pagination, and custom column manipulation.
//
// Security features:
//   - Validates all column names to prevent SQL injection, including Options.DefaultOrders
//   - Groups global-search OR conditions so they cannot escape the caller's own WHERE clauses
//   - Escapes LIKE wildcards in the search value
//   - Resolves per-column search (columns[i][search][value]) through
//     Options.ColumnSearch (or, if unset, the orderable map), so a client
//     cannot filter on a column the caller did not explicitly expose
//   - Caps search values at Options.MaxSearchLength runes
//   - Enforces a bounded page size (Options.DefaultPageSize/MaxPageSize)
//   - Propagates the request context, so a cancelled request stops its query
//
// Parameters:
//   - c: Gin context containing request parameters
//   - query: GORM query builder (can include WHERE clauses, JOINs, etc.)
//   - dest: Pointer to a slice where results will be stored
//   - searchable: List of columns that support global search
//   - orderable: Mapping between frontend column names and database columns
//   - opts: Optional column customizations (add/edit/remove/index/default order)
//
// Returns a dto.Datatables response structure compatible with DataTables JSON format,
// or an error if validation fails or database operations fail.
//
// Example:
//
//	var users []User
//	result, err := datatables.OfReturn(
//	    c,
//	    db.Model(&User{}),
//	    &users,
//	    []string{"name", "email"},
//	    map[string]string{"name": "name", "email": "email", "created": "created_at"},
//	    datatables.NewOptions().WithDefaultOrder("created_at", "desc"),
//	)
func OfReturn[T any](
	c *gin.Context,
	query *gorm.DB,
	dest *[]T,
	searchable []string,
	orderable map[string]string,
	opts Options,
) (dto.Datatables, error) {
	if err := validateSearchableColumns(searchable); err != nil {
		return dto.Datatables{}, err
	}
	if err := validateOrderableColumns(orderable); err != nil {
		return dto.Datatables{}, err
	}
	if err := validateDefaultOrders(opts.DefaultOrders); err != nil {
		return dto.Datatables{}, err
	}
	if err := validateOptionColumns(opts); err != nil {
		return dto.Datatables{}, err
	}

	opts = opts.normalized()

	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	query = query.WithContext(ctx)

	params := ParseParams(c, opts)

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return dto.Datatables{}, fmt.Errorf("datatables: count total: %w", err)
	}

	filteredQuery := query.Session(&gorm.Session{})
	if params.Search != "" && len(searchable) > 0 {
		filteredQuery = applySearch(filteredQuery, searchable, params.Search)
	}
	if len(params.ColumnSearches) > 0 {
		filteredQuery = applyColumnSearch(filteredQuery, columnSearchMap(opts, orderable), params.ColumnSearches, opts.logger())
	}

	var filtered int64
	if params.Search == "" && len(params.ColumnSearches) == 0 {
		filtered = total
	} else if err := filteredQuery.Count(&filtered).Error; err != nil {
		return dto.Datatables{}, fmt.Errorf("datatables: count filtered: %w", err)
	}

	filteredQuery, err := applyOrdering(filteredQuery, params, orderable, opts, dest)
	if err != nil {
		return dto.Datatables{}, err
	}

	if params.Length == -1 && opts.UnlimitedPageSize {
		filteredQuery = filteredQuery.Offset(params.Start)
	} else {
		limit := params.Length
		if limit <= 0 {
			limit = opts.DefaultPageSize
		}
		filteredQuery = filteredQuery.Offset(params.Start).Limit(limit)
	}

	if err := filteredQuery.Find(dest).Error; err != nil {
		return dto.Datatables{}, fmt.Errorf("datatables: find: %w", err)
	}

	rows, err := structToMapSlice(dest)
	if err != nil {
		return dto.Datatables{}, err
	}
	rows = applyOptions(rows, opts, params.Start)

	return dto.Datatables{
		Draw:            params.Draw,
		RecordsTotal:    total,
		RecordsFiltered: filtered,
		Data:            rows,
	}, nil
}

// applySearch adds a global search condition to the query, grouped in its
// own parenthesized clause so the OR chain across searchable columns cannot
// escape any WHERE conditions the caller already applied.
func applySearch(query *gorm.DB, searchable []string, searchValue string) *gorm.DB {
	pattern := "%" + escapeLike(searchValue) + "%"

	group := query.Session(&gorm.Session{NewDB: true})
	for i, col := range searchable {
		cond := likeCondition(query, col)
		if i == 0 {
			group = group.Where(cond, pattern)
		} else {
			group = group.Or(cond, pattern)
		}
	}

	return query.Where(group)
}

// likeCondition returns a dialect-appropriate LIKE expression for column col.
func likeCondition(query *gorm.DB, col string) string {
	if query == nil || query.Dialector == nil {
		return "LOWER(" + col + ") LIKE LOWER(?) ESCAPE '!'"
	}
	switch query.Dialector.Name() {
	case "postgres":
		return "CAST(" + col + " AS TEXT) ILIKE ? ESCAPE '!'"
	case "mysql":
		return "CAST(" + col + " AS CHAR) LIKE ? ESCAPE '!'"
	case "sqlite":
		return "LOWER(CAST(" + col + " AS TEXT)) LIKE LOWER(?) ESCAPE '!'"
	default:
		return "LOWER(" + col + ") LIKE LOWER(?) ESCAPE '!'"
	}
}

// escapeLike neutralizes LIKE wildcards so a search for "100%" or "a_b"
// is matched literally instead of being interpreted as a pattern.
func escapeLike(s string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(s)
}

// columnSearchMap returns the map that authorizes per-column search:
// opts.ColumnSearch when set, otherwise the orderable map (deprecated
// fallback, kept for v2 compatibility).
func columnSearchMap(opts Options, orderable map[string]string) map[string]string {
	if opts.ColumnSearch != nil {
		return opts.ColumnSearch
	}
	return orderable
}

// applyColumnSearch adds one AND-ed LIKE condition per requested
// columns[i][search][value], narrowing the result set beyond the global
// search. Each entry's frontend key is resolved through allowed (see
// columnSearchMap); a key not present there is skipped, since the server's
// map — not a client-side hint such as columns[i][searchable] — is the
// authority on which columns are exposed. Unlike applySearch, no OR
// grouping is needed here: every condition is independently AND-ed onto
// the query.
func applyColumnSearch(query *gorm.DB, allowed map[string]string, columnSearches []dto.ColumnSearch, logger *slog.Logger) *gorm.DB {
	for _, cs := range columnSearches {
		col, ok := allowed[cs.Column]
		if !ok {
			logger.Debug("datatables: column search key not allowed, skipping", "key", cs.Column)
			continue
		}

		pattern := "%" + escapeLike(cs.Value) + "%"
		query = query.Where(likeCondition(query, col), pattern)
	}

	return query
}

// applyOrdering adds ORDER BY clauses to the query, then appends a primary
// key tie-breaker so LIMIT/OFFSET paging stays stable.
//
// Every requested order column must be present in the orderable map. When
// one is not: if opts.StrictOrder is set, ErrInvalidOrderColumn is returned;
// otherwise the mismatch is logged and that column is skipped. If none of
// the requested columns resolve, opts.DefaultOrders is applied. If that is
// also empty, the query is left with no explicit ORDER BY beyond the
// tie-breaker, rather than silently returning rows in an undefined order
// under LIMIT/OFFSET.
func applyOrdering[T any](query *gorm.DB, params dto.Params, orderable map[string]string, opts Options, dest *[]T) (*gorm.DB, error) {
	var ordered []string

	for _, o := range params.Orders {
		col, ok := orderable[o.Column]
		if !ok {
			if opts.StrictOrder {
				return nil, fmt.Errorf("%w: %q", ErrInvalidOrderColumn, o.Column)
			}
			opts.logger().Debug("datatables: order column not in orderable map, skipping", "key", o.Column)
			continue
		}
		query = query.Order(col + " " + o.Dir)
		ordered = append(ordered, col)
	}

	if len(ordered) == 0 {
		for _, d := range opts.DefaultOrders {
			query = query.Order(d.Column + " " + d.Dir)
			ordered = append(ordered, d.Column)
		}
	}

	return withTieBreaker(query, dest, opts.TieBreaker, ordered), nil
}

// schemaCache backs schema.Parse for tie-breaker lookups, so resolving the
// primary key never mutates the caller's GORM statement.
var schemaCache sync.Map

// withTieBreaker appends a stable final ORDER BY column so LIMIT/OFFSET
// paging stays stable when the sorted column contains duplicate values.
// tieBreaker is Options.TieBreaker: "-" disables it, a column name is used
// as-is, and "" resolves the model's primary key. Nothing is appended for
// GROUP BY or DISTINCT queries, where an extra column changes the result,
// or when the column is already part of ordered.
//
// Statement.Schema is only populated when GORM executes, so the model is
// parsed separately from Statement.Model (set by the caller's own
// .Model(&T{}) call) or, failing that, from dest.
func withTieBreaker[T any](query *gorm.DB, dest *[]T, tieBreaker string, ordered []string) *gorm.DB {
	if tieBreaker == "-" {
		return query
	}
	stmt := query.Statement
	if _, hasGroup := stmt.Clauses["GROUP BY"]; hasGroup {
		return query
	}
	if stmt.Distinct {
		return query
	}

	if tieBreaker != "" {
		if slices.Contains(ordered, tieBreaker) {
			return query
		}
		return query.Order(tieBreaker)
	}

	model := stmt.Model
	if model == nil {
		model = dest
	}
	sch, err := schema.Parse(model, &schemaCache, query.NamingStrategy)
	if err != nil || sch == nil || sch.PrioritizedPrimaryField == nil {
		return query
	}
	pk := sch.PrioritizedPrimaryField.DBName
	if slices.Contains(ordered, pk) || slices.Contains(ordered, sch.Table+"."+pk) {
		return query
	}

	return query.Order(clause.OrderByColumn{
		Column: clause.Column{Table: clause.CurrentTable, Name: pk},
	})
}
