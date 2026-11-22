package datatables

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bonarizki/gin-datatables/v2/datatables/dto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"
)

// testUser is the model used across processor tests. It has a primary key
// so withTieBreaker has something to append.
type testUser struct {
	ID       uint `gorm:"primarykey"`
	Name     string
	Code     string
	TenantID string
}

func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("failed to open dry-run db: %v", err)
	}
	return db
}

func testGinContext(rawQuery string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?"+rawQuery, nil)
	return c
}

// TestApplySearch_DoesNotLeakAcrossCallerFilter is the regression test for
// the search data-leak fix: a caller-supplied WHERE clause (e.g. tenant
// isolation) must not be escaped by the OR chain across searchable columns.
func TestApplySearch_DoesNotLeakAcrossCallerFilter(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	base := db.Model(&testUser{}).Where("tenant_id = ?", "A")
	q := applySearch(base.Session(&gorm.Session{}), []string{"name", "code"}, "x")
	stmt := q.Find(&out).Statement

	sql := stmt.SQL.String()

	if !strings.Contains(sql, "tenant_id = ?") {
		t.Fatalf("expected caller's WHERE clause to survive, got SQL: %s", sql)
	}

	// The OR chain across searchable columns must be grouped in its own
	// parentheses so it cannot bind more loosely than the caller's AND.
	if !strings.Contains(sql, "(LOWER(name)") && !strings.Contains(sql, "(lower(`name`)") {
		t.Fatalf("expected the search OR chain to be parenthesized, got SQL: %s", sql)
	}

	// A naive `AND cond1 OR cond2` (no parens) is the exact shape of the bug
	// this test guards against.
	if regexpUngroupedOr(sql) {
		t.Fatalf("search conditions are not grouped, tenant filter can be bypassed: %s", sql)
	}
}

// regexpUngroupedOr reports whether sql contains an "OR" at parenthesis
// depth zero after the WHERE clause — the signature of the data-leak bug,
// where a bare OR binds more loosely than the caller's AND instead of being
// grouped in its own parenthesized clause.
func regexpUngroupedOr(sql string) bool {
	idx := strings.Index(sql, "WHERE")
	if idx == -1 {
		return false
	}

	rest := sql[idx:]
	depth := 0
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && i+2 <= len(rest) && rest[i:i+2] == "OR" {
			return true
		}
	}
	return false
}

func TestApplySearch_EscapesLikeWildcards(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	q := applySearch(db.Model(&testUser{}).Session(&gorm.Session{}), []string{"name"}, "100%_x\\y")
	stmt := q.Find(&out).Statement

	if len(stmt.Vars) == 0 {
		t.Fatalf("expected bound vars, got none")
	}
	pattern, ok := stmt.Vars[0].(string)
	if !ok {
		t.Fatalf("expected string var, got %T", stmt.Vars[0])
	}
	if pattern != `%100!%!_x\y%` {
		t.Errorf("expected escaped LIKE pattern, got %q", pattern)
	}
}

func TestApplySearch_SingleColumn(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	q := applySearch(db.Model(&testUser{}).Session(&gorm.Session{}), []string{"name"}, "x")
	sql := q.Find(&out).Statement.SQL.String()

	if strings.Contains(sql, "OR") {
		t.Errorf("single searchable column should not produce an OR: %s", sql)
	}
}

// TestApplyColumnSearch_KnownColumnAddsAndedLikeClause verifies that a
// columns[i][search][value] entry resolved through the orderable map
// produces an AND-ed, escaped LIKE condition — not grouped with OR, since
// each per-column filter is independent.
func TestApplyColumnSearch_KnownColumnAddsAndedLikeClause(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	orderable := map[string]string{"name": "name"}
	searches := []dto.ColumnSearch{{Column: "name", Value: "100%_x"}}

	q := applyColumnSearch(db.Model(&testUser{}).Session(&gorm.Session{}), orderable, searches, discardLogger)
	stmt := q.Find(&out).Statement
	sql := stmt.SQL.String()

	if !strings.Contains(sql, "LOWER(name) LIKE LOWER(?)") {
		t.Fatalf("expected a LOWER(...) LIKE LOWER(?) clause, got SQL: %s", sql)
	}
	if regexpUngroupedOr(sql) {
		t.Errorf("column search must not introduce an ungrouped OR: %s", sql)
	}

	if len(stmt.Vars) == 0 {
		t.Fatalf("expected bound vars, got none")
	}
	pattern, ok := stmt.Vars[len(stmt.Vars)-1].(string)
	if !ok {
		t.Fatalf("expected string var, got %T", stmt.Vars[len(stmt.Vars)-1])
	}
	if pattern != `%100!%!_x%` {
		t.Errorf("expected escaped LIKE pattern, got %q", pattern)
	}
}

// TestApplyColumnSearch_UnknownColumnIsSkipped verifies that a search
// value for a frontend key not present in the orderable map is ignored
// rather than trusted, since the orderable map is the authority on which
// columns are exposed.
func TestApplyColumnSearch_UnknownColumnIsSkipped(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	searches := []dto.ColumnSearch{{Column: "secret_field", Value: "x"}}

	q := applyColumnSearch(db.Model(&testUser{}).Session(&gorm.Session{}), map[string]string{"name": "name"}, searches, discardLogger)
	sql := q.Find(&out).Statement.SQL.String()

	if strings.Contains(sql, "LIKE") {
		t.Errorf("expected no LIKE clause for an unresolved column, got SQL: %s", sql)
	}
}

// TestApplyColumnSearch_MultipleColumnsAreAllAnded verifies that several
// per-column search values are all applied as independent AND conditions.
func TestApplyColumnSearch_MultipleColumnsAreAllAnded(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	orderable := map[string]string{"name": "name", "code": "code"}
	searches := []dto.ColumnSearch{
		{Column: "name", Value: "john"},
		{Column: "code", Value: "abc"},
	}

	q := applyColumnSearch(db.Model(&testUser{}).Session(&gorm.Session{}), orderable, searches, discardLogger)
	sql := q.Find(&out).Statement.SQL.String()

	if !strings.Contains(sql, "LOWER(name) LIKE LOWER(?)") || !strings.Contains(sql, "LOWER(code) LIKE LOWER(?)") {
		t.Fatalf("expected both column search clauses, got SQL: %s", sql)
	}
	if regexpUngroupedOr(sql) {
		t.Errorf("multiple column searches must stay AND-ed, not OR-ed: %s", sql)
	}
}

// TestApplyColumnSearch_DoesNotLeakAcrossCallerFilter mirrors the global
// search regression test: a caller-supplied WHERE clause must survive
// alongside a per-column search condition, and the two must combine with
// AND rather than reintroducing the ungrouped-OR data leak.
func TestApplyColumnSearch_DoesNotLeakAcrossCallerFilter(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	base := db.Model(&testUser{}).Where("tenant_id = ?", "A")
	q := applyColumnSearch(base.Session(&gorm.Session{}), map[string]string{"name": "name"}, []dto.ColumnSearch{{Column: "name", Value: "x"}}, discardLogger)
	sql := q.Find(&out).Statement.SQL.String()

	if !strings.Contains(sql, "tenant_id = ?") {
		t.Fatalf("expected caller's WHERE clause to survive, got SQL: %s", sql)
	}
	if regexpUngroupedOr(sql) {
		t.Fatalf("column search combined with a caller filter must not produce an ungrouped OR: %s", sql)
	}
}

func TestApplyOrdering_RequestedColumnResolvedAndTieBroken(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	params := dto.Params{Orders: []dto.Order{{Column: "name", Dir: "desc"}}}
	orderable := map[string]string{"name": "users.name"}
	opts := NewOptions()

	q, err := applyOrdering(db.Model(&testUser{}).Session(&gorm.Session{}), params, orderable, opts, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql := q.Find(&out).Statement.SQL.String()
	if !strings.Contains(sql, "ORDER BY users.name desc") {
		t.Errorf("expected requested order to be applied, got SQL: %s", sql)
	}
	if !strings.Contains(sql, "`id`") && !strings.Contains(sql, ".id") {
		t.Errorf("expected primary key tie-breaker to be appended, got SQL: %s", sql)
	}
}

func TestApplyOrdering_NoValidOrderStillGetsTieBreaker(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	params := dto.Params{} // no request order, no default order
	opts := NewOptions()

	q, err := applyOrdering(db.Model(&testUser{}).Session(&gorm.Session{}), params, map[string]string{}, opts, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql := q.Offset(0).Limit(10).Find(&out).Statement.SQL.String()
	if !strings.Contains(sql, "ORDER BY") {
		t.Fatalf("expected a tie-breaker ORDER BY even with no user/default order, got SQL: %s", sql)
	}
}

func TestApplyOrdering_FallsBackToDefaultOrder(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	// The request's order key does not exist in orderable.
	params := dto.Params{Orders: []dto.Order{{Column: "unknown", Dir: "asc"}}}
	opts := NewOptions().WithDefaultOrder("created_at", "desc")

	q, err := applyOrdering(db.Model(&testUser{}).Session(&gorm.Session{}), params, map[string]string{"name": "name"}, opts, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql := q.Find(&out).Statement.SQL.String()
	if !strings.Contains(sql, "created_at desc") {
		t.Errorf("expected default order to be applied, got SQL: %s", sql)
	}
}

func TestApplyOrdering_StrictOrderReturnsError(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	params := dto.Params{Orders: []dto.Order{{Column: "unknown", Dir: "asc"}}}
	opts := NewOptions().WithStrictOrder(true)

	_, err := applyOrdering(db.Model(&testUser{}).Session(&gorm.Session{}), params, map[string]string{"name": "name"}, opts, &out)
	if !errors.Is(err, ErrInvalidOrderColumn) {
		t.Errorf("expected ErrInvalidOrderColumn, got %v", err)
	}
}

func TestApplyOrdering_MultiColumn(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	params := dto.Params{Orders: []dto.Order{
		{Column: "status", Dir: "asc"},
		{Column: "created", Dir: "desc"},
	}}
	orderable := map[string]string{"status": "status", "created": "created_at"}
	opts := NewOptions()

	q, err := applyOrdering(db.Model(&testUser{}).Session(&gorm.Session{}), params, orderable, opts, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql := q.Find(&out).Statement.SQL.String()
	if !strings.Contains(sql, "ORDER BY status asc,created_at desc") {
		t.Errorf("expected multi-column ORDER BY, got SQL: %s", sql)
	}
}

// TestOfReturn_EndToEnd exercises the full pipeline against a DryRun
// connection: no real database is required, and OfReturn must not error or
// panic while wiring search, ordering, pagination, and the converter
// together.
func TestOfReturn_EndToEnd(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	c := testGinContext("search[value]=x&order[0][column]=name&order[0][dir]=desc&length=5&start=0")

	result, err := OfReturn(
		c,
		db.Model(&testUser{}).Where("tenant_id = ?", "A"),
		&out,
		[]string{"name", "code"},
		map[string]string{"name": "name", "code": "code"},
		NewOptions(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Draw != 1 {
		t.Errorf("expected default draw=1, got %d", result.Draw)
	}
}

// TestOfReturn_ColumnSearchEndToEnd exercises columns[i][data]/
// columns[i][search][value] through the full OfReturn pipeline.
func TestOfReturn_ColumnSearchEndToEnd(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser

	c := testGinContext("columns[0][data]=name&columns[0][search][value]=john&length=5&start=0")

	result, err := OfReturn(
		c,
		db.Model(&testUser{}),
		&out,
		[]string{"name", "code"},
		map[string]string{"name": "name", "code": "code"},
		NewOptions(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Draw != 1 {
		t.Errorf("expected default draw=1, got %d", result.Draw)
	}
}

func TestOfReturn_InvalidSearchableColumnIsRejected(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser
	c := testGinContext("")

	_, err := OfReturn(c, db.Model(&testUser{}), &out, []string{"name; DROP TABLE users"}, map[string]string{}, NewOptions())
	if !errors.Is(err, ErrInvalidColumnName) {
		t.Errorf("expected ErrInvalidColumnName, got %v", err)
	}
}

func TestOfReturn_InvalidDefaultOrderIsRejected(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser
	c := testGinContext("")

	opts := NewOptions().WithDefaultOrder("created_at", "sideways")
	_, err := OfReturn(c, db.Model(&testUser{}), &out, nil, map[string]string{}, opts)
	if !errors.Is(err, ErrInvalidOrderDirection) {
		t.Errorf("expected ErrInvalidOrderDirection, got %v", err)
	}
}
