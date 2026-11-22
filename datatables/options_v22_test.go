package datatables

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/bonarizki/gin-datatables/v2/datatables/dto"
	"gorm.io/gorm"
)

func orderSQL(t *testing.T, params dto.Params, orderable map[string]string, opts Options) string {
	t.Helper()
	db := dryRunDB(t)
	var out []testUser
	q, err := applyOrdering(db.Model(&testUser{}).Session(&gorm.Session{}), params, orderable, opts, &out)
	if err != nil {
		t.Fatalf("applyOrdering: %v", err)
	}
	return q.Find(&out).Statement.SQL.String()
}

func TestTieBreaker_DisabledWithDash(t *testing.T) {
	sql := orderSQL(t, dto.Params{Orders: []dto.Order{{Column: "name", Dir: "asc"}}},
		map[string]string{"name": "name"}, NewOptions().WithTieBreaker("-"))
	if strings.Contains(sql, "id") {
		t.Errorf("tie-breaker disabled but SQL orders by id: %s", sql)
	}
}

func TestTieBreaker_CustomColumn(t *testing.T) {
	sql := orderSQL(t, dto.Params{Orders: []dto.Order{{Column: "name", Dir: "asc"}}},
		map[string]string{"name": "name"}, NewOptions().WithTieBreaker("code"))
	if !strings.Contains(sql, "ORDER BY name asc,code") {
		t.Errorf("want custom tie-breaker after the requested order, got: %s", sql)
	}
}

func TestTieBreaker_SkippedWhenPrimaryKeyAlreadyOrdered(t *testing.T) {
	sql := orderSQL(t, dto.Params{Orders: []dto.Order{{Column: "id", Dir: "desc"}}},
		map[string]string{"id": "id"}, NewOptions())
	if strings.Count(sql, "id") != 1 {
		t.Errorf("primary key ordered twice: %s", sql)
	}
}

func TestTieBreaker_DoesNotMutateCallerStatement(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser
	base := db.Model(&testUser{}).Session(&gorm.Session{})
	if _, err := applyOrdering(base, dto.Params{}, nil, NewOptions(), &out); err != nil {
		t.Fatal(err)
	}
	if base.Statement.Schema != nil || base.Statement.Table != "" {
		t.Errorf("caller statement mutated: Schema=%v Table=%q", base.Statement.Schema, base.Statement.Table)
	}
}

func TestOfReturn_InvalidTieBreakerRejected(t *testing.T) {
	db := dryRunDB(t)
	var out []testUser
	_, err := OfReturn(testGinContext(""), db.Model(&testUser{}), &out, nil, nil,
		NewOptions().WithTieBreaker("id; DROP TABLE x"))
	if !errors.Is(err, ErrInvalidColumnName) {
		t.Errorf("err = %v, want ErrInvalidColumnName", err)
	}
}

func TestColumnSearch_AllowListOverridesOrderable(t *testing.T) {
	allowed := columnSearchMap(NewOptions().WithColumnSearch(map[string]string{"code": "code"}),
		map[string]string{"name": "name", "code": "code"})

	db := dryRunDB(t)
	var out []testUser
	q := applyColumnSearch(db.Model(&testUser{}).Session(&gorm.Session{}), allowed,
		[]dto.ColumnSearch{{Column: "name", Value: "x"}, {Column: "code", Value: "y"}}, discardLogger)
	sql := q.Find(&out).Statement.SQL.String()
	if strings.Contains(sql, "name") || !strings.Contains(sql, "code") {
		t.Errorf("want only code filtered, got: %s", sql)
	}
}

func TestColumnSearch_EmptyMapDisables(t *testing.T) {
	allowed := columnSearchMap(NewOptions().WithColumnSearch(map[string]string{}),
		map[string]string{"name": "name"})
	if allowed == nil || len(allowed) != 0 {
		t.Errorf("empty WithColumnSearch should yield an empty, non-nil map, got %v", allowed)
	}
}

func TestLogger_ReceivesSkippedKeys(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	orderSQL(t, dto.Params{Orders: []dto.Order{{Column: "nope", Dir: "asc"}}},
		map[string]string{}, NewOptions().WithLogger(logger))
	if !strings.Contains(buf.String(), "key=nope") {
		t.Errorf("logger output = %q, want a record for key=nope", buf.String())
	}
}

func TestParseParams_TruncatesSearch(t *testing.T) {
	long := strings.Repeat("é", 300)
	c := testGinContext("search[value]=" + long + "&columns[0][data]=name&columns[0][search][value]=" + long)
	p := ParseParams(c, NewOptions())
	if n := len([]rune(p.Search)); n != 256 {
		t.Errorf("global search length = %d runes, want 256", n)
	}
	if n := len([]rune(p.ColumnSearches[0].Value)); n != 256 {
		t.Errorf("column search length = %d runes, want 256", n)
	}
}

func TestDefaultOrder_DirectionIsCaseInsensitive(t *testing.T) {
	opts := NewOptions().WithDefaultOrder("name", "Desc").WithDefaultOrder("code", "aSc")
	if err := validateDefaultOrders(opts.DefaultOrders); err != nil {
		t.Fatalf("mixed-case directions rejected: %v", err)
	}
	if opts.DefaultOrders[0].Dir != "desc" || opts.DefaultOrders[1].Dir != "asc" {
		t.Errorf("directions not normalized: %+v", opts.DefaultOrders)
	}
}
