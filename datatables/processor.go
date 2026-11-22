package datatables

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bonarizki/gin-datatables/v2/datatables/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OfReturn executes the core DataTables server-side logic.
// It supports searching, ordering, pagination, and custom column manipulation.
//
// Security features:
//   - Validates all column names to prevent SQL injection, including Options.DefaultOrders
//   - Groups global-search OR conditions so they cannot escape the caller's own WHERE clauses
//   - Escapes LIKE wildcards in the search value
//   - Resolves per-column search (columns[i][search][value]) through the
//     same orderable map used for sorting, so a client cannot filter on a
//     column the caller did not explicitly expose
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
		filteredQuery = applyColumnSearch(filteredQuery, orderable, params.ColumnSearches)
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

// applyColumnSearch adds one AND-ed LIKE condition per requested
// columns[i][search][value], narrowing the result set beyond the global
// search. Each entry's frontend key is resolved through the same orderable
// map used for sorting; a key not present there is not filterable this way
// and is skipped, since the orderable map — not a client-side hint such as
// columns[i][searchable] — is the authority on which columns are exposed.
// Unlike applySearch, no OR grouping is needed here: every condition is
// independently AND-ed onto the query.
func applyColumnSearch(query *gorm.DB, orderable map[string]string, columnSearches []dto.ColumnSearch) *gorm.DB {
	for _, cs := range columnSearches {
		col, ok := orderable[cs.Column]
		if !ok {
			log.Printf("datatables: column search key %q not found in orderable map, skipping", cs.Column)
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
	applied := false

	for _, o := range params.Orders {
		col, ok := orderable[o.Column]
		if !ok {
			if opts.StrictOrder {
				return nil, fmt.Errorf("%w: %q", ErrInvalidOrderColumn, o.Column)
			}
			log.Printf("datatables: order column %q not found in orderable map, skipping", o.Column)
			continue
		}
		query = query.Order(col + " " + o.Dir)
		applied = true
	}

	if !applied {
		for _, d := range opts.DefaultOrders {
			query = query.Order(d.Column + " " + d.Dir)
		}
	}

	return withTieBreaker(query, dest), nil
}

// withTieBreaker appends the primary key so LIMIT/OFFSET paging stays stable
// when the sorted column contains duplicate values.
//
// Reading query.Statement.Schema before the query executes is a no-op: GORM
// only populates it inside Execute. To get a usable schema here, the model
// is parsed explicitly from Statement.Model (set by the caller's own
// .Model(&T{}) call) or, failing that, from dest.
func withTieBreaker[T any](query *gorm.DB, dest *[]T) *gorm.DB {
	stmt := query.Statement
	if _, hasGroup := stmt.Clauses["GROUP BY"]; hasGroup {
		return query
	}
	if stmt.Distinct {
		return query
	}

	model := stmt.Model
	if model == nil {
		model = dest
	}

	if err := stmt.Parse(model); err != nil || stmt.Schema == nil {
		return query
	}

	pk := stmt.Schema.PrioritizedPrimaryField
	if pk == nil {
		return query
	}

	return query.Order(clause.OrderByColumn{
		Column: clause.Column{Table: clause.CurrentTable, Name: pk.DBName},
	})
}
