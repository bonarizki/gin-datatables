package datatables

import (
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/bonarizki/gin-datatables/v2/datatables/dto"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func mustParseParams(t *testing.T, rawQuery string, opts Options) dto.Params {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?"+rawQuery, nil)
	return ParseParams(c, opts)
}

func TestParseParams_Orders(t *testing.T) {
	opts := NewOptions()

	tests := []struct {
		name     string
		query    string
		expected []dto.Order
	}{
		{
			name:     "numeric index resolved via columns[i][data] (official contract)",
			query:    "order[0][column]=2&order[0][dir]=desc&columns[2][data]=code",
			expected: []dto.Order{{Column: "code", Dir: "desc"}},
		},
		{
			name:     "direct key (shorthand contract)",
			query:    "order[0][column]=name&order[0][dir]=asc",
			expected: []dto.Order{{Column: "name", Dir: "asc"}},
		},
		{
			name:     "numeric index with no matching columns[i][data] falls back to shorthand",
			query:    "order[0][column]=2&order[0][dir]=asc",
			expected: []dto.Order{{Column: "2", Dir: "asc"}},
		},
		{
			name:     "order param entirely absent",
			query:    "start=0",
			expected: nil,
		},
		{
			name:     "order[0][column] present but empty is skipped",
			query:    "order[0][column]=&columns[0][data]=name",
			expected: nil,
		},
		{
			name:     "garbage dir defaults to asc",
			query:    "order[0][column]=name&order[0][dir]=sideways",
			expected: []dto.Order{{Column: "name", Dir: "asc"}},
		},
		{
			name:     "dir omitted defaults to asc",
			query:    "order[0][column]=name",
			expected: []dto.Order{{Column: "name", Dir: "asc"}},
		},
		{
			name:  "multi-column order (shift-click sort)",
			query: "order[0][column]=status&order[0][dir]=asc&order[1][column]=created&order[1][dir]=desc",
			expected: []dto.Order{
				{Column: "status", Dir: "asc"},
				{Column: "created", Dir: "desc"},
			},
		},
		{
			name:  "multi-column order stops at first missing index",
			query: "order[0][column]=a&order[2][column]=c",
			expected: []dto.Order{
				{Column: "a", Dir: "asc"},
			},
		},
		{
			name:     "order[i][name] takes priority over numeric index",
			query:    "order[0][column]=2&order[0][name]=status&order[0][dir]=desc&columns[2][data]=code",
			expected: []dto.Order{{Column: "status", Dir: "desc"}},
		},
		{
			name:     "order[i][name] takes priority over shorthand key",
			query:    "order[0][column]=name&order[0][name]=status&order[0][dir]=asc",
			expected: []dto.Order{{Column: "status", Dir: "asc"}},
		},
		{
			name:     "order[i][name] absent falls back to existing resolution",
			query:    "order[0][column]=name&order[0][dir]=asc",
			expected: []dto.Order{{Column: "name", Dir: "asc"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustParseParams(t, tt.query, opts).Orders

			if len(got) != len(tt.expected) {
				t.Fatalf("Orders = %+v, want %+v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("Orders[%d] = %+v, want %+v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestParseParams_Length(t *testing.T) {
	defaultOpts := NewOptions()
	unlimitedOpts := NewOptions().WithUnlimitedPageSize(true)
	customOpts := NewOptions().WithPageSize(25, 100)

	tests := []struct {
		name     string
		query    string
		opts     Options
		expected int
	}{
		{"absent falls back to DefaultPageSize", "", defaultOpts, 10},
		{"zero falls back to DefaultPageSize", "length=0", defaultOpts, 10},
		{"garbage falls back to DefaultPageSize", "length=abc", defaultOpts, 10},
		{"negative (not -1) falls back to DefaultPageSize", "length=-50", defaultOpts, 10},
		{"above MaxPageSize is clamped", "length=99999", defaultOpts, 500},
		{"within bounds is kept as-is", "length=50", defaultOpts, 50},
		{"-1 without UnlimitedPageSize falls back to DefaultPageSize", "length=-1", defaultOpts, 10},
		{"-1 with UnlimitedPageSize is preserved", "length=-1", unlimitedOpts, -1},
		{"custom default/max are honored", "length=0", customOpts, 25},
		{"custom max clamps", "length=1000", customOpts, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustParseParams(t, tt.query, tt.opts).Length
			if got != tt.expected {
				t.Errorf("Length = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestParseParams_StartAndDraw(t *testing.T) {
	opts := NewOptions()

	tests := []struct {
		name          string
		query         string
		expectedStart int
		expectedDraw  int64
	}{
		{"defaults", "", 0, 1},
		{"valid start and draw", "start=20&draw=3", 20, 3},
		{"negative start clamps to 0", "start=-50", 0, 1},
		{"garbage start clamps to 0", "start=abc", 0, 1},
		{"garbage draw falls back to 1", "draw=abc", 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := mustParseParams(t, tt.query, opts)
			if p.Start != tt.expectedStart {
				t.Errorf("Start = %d, want %d", p.Start, tt.expectedStart)
			}
			if p.Draw != tt.expectedDraw {
				t.Errorf("Draw = %d, want %d", p.Draw, tt.expectedDraw)
			}
		})
	}
}

func TestParseParams_Search(t *testing.T) {
	opts := NewOptions()

	p := mustParseParams(t, "search[value]=100%25_x", opts)
	if p.Search != "100%_x" {
		t.Errorf("Search = %q, want %q", p.Search, "100%_x")
	}

	p = mustParseParams(t, "", opts)
	if p.Search != "" {
		t.Errorf("Search = %q, want empty", p.Search)
	}
}

func TestResolveOrderColumn(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?columns[2][data]=code", nil)

	tests := []struct {
		name      string
		raw       string
		orderName string
		expected  string
	}{
		{name: "empty raw and no order name returns empty", raw: "", orderName: "", expected: ""},
		{name: "numeric with matching columns[i][data]", raw: "2", orderName: "", expected: "code"},
		{name: "numeric with no matching columns[i][data] falls back to raw", raw: "5", orderName: "", expected: "5"},
		{name: "shorthand key returned as-is", raw: "orders.status", orderName: "", expected: "orders.status"},
		{name: "order name takes priority over numeric raw", raw: "2", orderName: "status", expected: "status"},
		{name: "order name takes priority over shorthand raw", raw: "orders.status", orderName: "code", expected: "code"},
		{name: "order name takes priority even with empty raw", raw: "", orderName: "status", expected: "status"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveOrderColumn(c, tt.raw, tt.orderName)
			if got != tt.expected {
				t.Errorf("resolveOrderColumn(%q, %q) = %q, want %q", tt.raw, tt.orderName, got, tt.expected)
			}
		})
	}
}

func TestParseParams_ColumnSearches(t *testing.T) {
	opts := NewOptions()

	tests := []struct {
		name     string
		query    string
		expected []dto.ColumnSearch
	}{
		{
			name:     "no columns sent",
			query:    "start=0",
			expected: nil,
		},
		{
			name:     "single column with a search value",
			query:    "columns[0][data]=name&columns[0][search][value]=john",
			expected: []dto.ColumnSearch{{Column: "name", Value: "john"}},
		},
		{
			name: "empty search value is skipped, later columns still read",
			query: "columns[0][data]=name&columns[0][search][value]=" +
				"&columns[1][data]=email&columns[1][search][value]=example.com",
			expected: []dto.ColumnSearch{{Column: "email", Value: "example.com"}},
		},
		{
			name:     "missing columns[i][data] stops the loop",
			query:    "columns[0][data]=name&columns[0][search][value]=john&columns[2][data]=email&columns[2][search][value]=x",
			expected: []dto.ColumnSearch{{Column: "name", Value: "john"}},
		},
		{
			name:     "multiple columns with search values",
			query:    "columns[0][data]=name&columns[0][search][value]=john&columns[1][data]=email&columns[1][search][value]=example.com",
			expected: []dto.ColumnSearch{{Column: "name", Value: "john"}, {Column: "email", Value: "example.com"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mustParseParams(t, tt.query, opts).ColumnSearches

			if len(got) != len(tt.expected) {
				t.Fatalf("ColumnSearches = %+v, want %+v", got, tt.expected)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("ColumnSearches[%d] = %+v, want %+v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestParseColumnSearches_MaxColumnsCap(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	query := ""
	for i := 0; i < maxColumns+5; i++ {
		if i > 0 {
			query += "&"
		}
		idx := strconv.Itoa(i)
		query += "columns[" + idx + "][data]=col" + idx + "&columns[" + idx + "][search][value]=v"
	}
	c.Request = httptest.NewRequest("GET", "/?"+query, nil)

	got := parseColumnSearches(c)
	if len(got) != maxColumns {
		t.Errorf("len(ColumnSearches) = %d, want %d (maxColumns cap)", len(got), maxColumns)
	}
}
