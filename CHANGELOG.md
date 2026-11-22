# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [2.2.0] - 2026-11-23

Additive release. No existing call needs to change.

### ✅ Added

- `WithTieBreaker(col)`: choose the column appended last to `ORDER BY`
  for stable paging, or `"-"` to disable it. The primary key is no longer
  appended when it is already ordered, nor for `GROUP BY`/`DISTINCT`
  queries, and resolving it no longer mutates the caller's statement.
- `WithColumnSearch(map)`: explicit allow-list for
  `columns[i][search][value]`. An empty map disables per-column search.
  Without it, keys still resolve through `orderable`; that fallback is
  deprecated and becomes opt-in in v3.
- `WithLogger(*slog.Logger)`: skipped order/search keys are logged at
  debug level to the injected logger. The package no longer writes to the
  standard library's global logger.
- `WithMaxSearchLength(n)` (default 256 runes) caps global and per-column
  search values.
- `ExampleOfReturn` and an in-memory SQLite integration suite.

### 🔧 Changed

- Order directions are validated case-insensitively, and
  `WithDefaultOrder` stores them lowercase.
- A nil pointer in the destination slice is skipped instead of producing
  an empty row.

---

## [2.1.1] - 2026-11-23

Urgent fixes, tagged on the commit before the 2.2.0 additions.

> ⚠️ **Review your structs before upgrading.** Fields tagged
> `json:",omitempty"` (empty name) used to be dropped from the output by
> mistake. They now appear under their Go field name, like
> `encoding/json`. If such a field is sensitive (for example
> ``Password string `json:",omitempty"` ``), hide it with `Remove(...)`.

> ⚠️ **A zero-value or misconfigured `Options` used to return every row.**
> `Options{}` (or a zero `DefaultPageSize`/`MaxPageSize`) disabled
> `LIMIT`. Defaults of 10 and 500 now apply, so callers that relied on the
> unlimited behavior must opt in with `WithUnlimitedPageSize(true)` and
> `length=-1`.

### 🔴 Fixed

- Search uses `ESCAPE '!'` instead of `ESCAPE '\'`, which was a syntax
  error under MySQL's default `sql_mode`.
- Search casts columns to text per dialect (Postgres `ILIKE`, MySQL
  `CAST(... AS CHAR)`), so searching numeric or timestamp columns no longer
  fails on Postgres.
- `Add`, `Edit`, `Remove` and `WithDefaultOrder` no longer share maps or
  slices with the `Options` they were called on, and work on a zero-value
  `Options`.
- DataTables parameters are read from POST form bodies as well as the
  query string.
- The primary-key tie-breaker is skipped for `GROUP BY` and `DISTINCT`
  queries.

### 🔧 Changed

- Minimum Go version lowered from 1.26 to 1.23.
- `retract v2.1.0`: that tag was published with the old module path. There
  is no `v2.0.0` under `github.com/bonarizki/gin-datatables/v2`.
- MIT `LICENSE` file added.

---

## [2.1.0] - 2026-08-21

Additive, non-breaking release. Closes two remaining gaps against the
official DataTables server-side protocol and documents which parts of the
protocol are intentionally not implemented.

### ✅ Added

- **Per-column search**: `columns[i][search][value]` is now read and
  applied as an `AND`-ed, escaped `LIKE` condition per column, on top of
  global search and any caller-supplied `WHERE`. The frontend key is
  resolved through the existing `orderable` map passed to `OfReturn` — no
  new parameter or `Options` field, so no caller code changes are needed to
  adopt it.
- **`order[i][name]` support**: when the client sends a column's configured
  name (via `columns.name`), it now takes priority over resolving
  `order[i][column]` as a numeric index or a shorthand key.
- `dto.ColumnSearch` and `dto.Params.ColumnSearches`.
- `parser_test.go`/`processor_test.go` coverage for both additions. Package
  coverage: 92.8% → 93.5%.

### 🔧 Changed

- Minimum Go version raised from 1.23 to 1.26.

### 📖 Documented (not implemented, by design)

- `search[regex]` / `columns[i][search][regex]` — regex search is not
  supported. SQL regex syntax is not portable across database engines, and
  an untrusted regex is a denial-of-service vector (catastrophic
  backtracking).
- `columns[i][searchable]` / `columns[i][orderable]` — these client-sent
  flags are never read. The `searchable`/`orderable` arguments passed to
  `OfReturn` remain the sole authority on what a client may search or sort
  by; trusting client-side hints for that decision would let a client
  widen its own access.

---

## [2.0.0] - 2026-08-21

Fixes a cross-tenant data leak in global search, restores spec-compliant
ordering (`order[0][column]` is a numeric index into `columns[]`, not a
column name), bounds pagination, and fixes the response/converter output
contract. See [MIGRATING.md](MIGRATING.md) for the full upgrade guide.

### 🔴 Security fixes

- **Fixed:** `applySearch` chained `.Or()` conditions directly onto the
  caller's query without grouping. `AND` binds tighter than `OR`, so a
  caller-supplied filter (e.g. tenant isolation) could be bypassed by any
  row matching the second or later searchable column. The OR chain is now
  wrapped in its own parenthesized clause.
- **Fixed:** search values were not escaped for LIKE wildcards, so a search
  for `100%` matched everything. `%`, `_`, and `\` are now escaped.
- **Fixed:** `length=0`, `length=abc`, or any unparseable/non-positive value
  disabled `LIMIT` entirely, returning the whole table. `length` is now
  always bounded by `Options.DefaultPageSize`/`MaxPageSize` unless the
  server explicitly opts into `WithUnlimitedPageSize(true)`.

### 🔴 Correctness fixes

- **Fixed:** `order[0][column]` was treated as a column name instead of the
  numeric index the DataTables spec sends. The `columns[i][data]` fallback
  could only run when the parameter was entirely absent, so a stock
  DataTables client's sort was silently ignored.
- **Fixed:** with no valid order and no default order, the query ran with
  no `ORDER BY` at all while `LIMIT`/`OFFSET` was still applied, making
  paged results non-deterministic (duplicated or missing rows across
  pages). A primary key tie-breaker is now appended automatically.
- **Fixed:** embedded struct fields (e.g. `gorm.Model`) were either dropped
  silently (unexported embed) or nested under the embedded type's name
  instead of being promoted to the root — so most GORM models never sent
  their `id` to the frontend. Embedded fields are now flattened like
  `encoding/json` does, with the outer struct's own fields taking
  precedence.
- **Fixed:** `datatables.JSON()` wrapped the DataTables payload in a
  `{success, message, data: {...}}` envelope. The stock jQuery DataTables
  client requires `draw`/`recordsTotal`/`recordsFiltered`/`data` at the
  response root; `JSON()` now writes `dto.Datatables` directly.
- **Fixed:** a non-struct or nil-element destination passed to the internal
  converter caused a `reflect` panic instead of returning `ErrInvalidData`.

### ✅ Added

- `Options.WithStrictOrder(bool)` — return `ErrInvalidOrderColumn` instead of
  silently skipping an unknown order key.
- `Options.WithPageSize(def, max int)` and `Options.WithUnlimitedPageSize(bool)`.
- Multi-column sort: `order[0]`, `order[1]`, ... are all read (shift-click
  sorting), not just `order[0]`.
- `datatables.ErrInvalidOrderColumn`, `ErrInvalidOrderDirection`.
- `*ValidationError` now implements `Unwrap()`, so `errors.Is(err,
  datatables.ErrInvalidColumnName)` works instead of requiring message
  parsing.
- `parser_test.go` and `processor_test.go`: table-driven `ParseParams` tests
  and `gorm.Config{DryRun: true}` SQL assertions for search grouping,
  ordering, and the tie-breaker. Package coverage: 57.4% → 92.8%.
- The request context (`c.Request.Context()`) now propagates into the
  query, so a cancelled request stops its query instead of running to
  completion.

### 🔧 Changed (breaking)

- Module path is now `github.com/bonarizki/gin-datatables/v2`.
- `ParseParams(c *gin.Context)` → `ParseParams(c *gin.Context, opts Options)`,
  needed to read the page-size bounds.
- `dto.Params.Order string` / `Dir string` → `dto.Params.Orders []dto.Order`.
- `Options.WithDefaultOrder(order string)` → `WithDefaultOrder(col, dir
  string)`. The free-form SQL string is gone; both arguments are validated.
- `Options.DefaultOrder string` → `Options.DefaultOrders []dto.Order`.
- `AddColumns`/`EditColumns` callbacks now receive the row as it stands
  after the index column and any earlier `AddColumns` entries, instead of
  the original unmodified row.

### 🗑️ Removed (breaking)

- `dto.SuccessResponse` and `dto.ResponseDatatables` (only used by the old
  `JSON()`/`JSONError()` envelope).
- `ErrColumnNotFound`, `ErrDefaultOrderColumn` sentinels — never returned by
  any code path; superseded by `ErrInvalidOrderColumn`.

---

## [Unreleased] - 2025-11-10 (v1.x API, superseded — see MIGRATING.md)

### ✅ Added

- **Security**: Column name validation to prevent SQL injection attacks
- **Feature**: `WithDefaultOrder()` method for configurable default ordering
- **Feature**: `JSONError()` helper function for consistent error responses
- **Error Types**: Sentinel errors (`ErrInvalidColumnName`, etc.) alongside
  `ValidationError`. Note: at this point in history `ValidationError` did not
  implement `Unwrap()`, so `errors.Is` against these sentinels never
  matched — fixed in v2.0.0.
- **Testing**: Unit tests covering 57.4% of statements, with no coverage for
  `parser.go`/`processor.go` (the search/order/pagination logic) — closed
  in v2.0.0, see above.
  - `validation_test.go` - Security validation tests
  - `converter_test.go` - Struct conversion tests
  - `options_test.go` - Options builder tests
  - `transformer_test.go` - Transformation tests
- **Documentation**: Extensive README with examples and API reference
- **Documentation**: `BACKWARD_COMPATIBILITY.md` guide
- **Examples**: `examples/basic_usage.go` demonstrating usage patterns

### 🔧 Changed

- **Refactor**: Split monolithic `datatables.go` into 9 modular files:
  - `datatables.go` - Package documentation (55 lines)
  - `processor.go` - Core OfReturn logic (133 lines)
  - `options.go` - Options builder (111 lines)
  - `parser.go` - Request parameter parsing (67 lines)
  - `converter.go` - Struct to map conversion (112 lines)
  - `transformer.go` - Column transformations (65 lines)
  - `validation.go` - Security validation (62 lines)
  - `errors.go` - Error types (28 lines)
  - `response.go` - JSON response helpers (42 lines)
- **Improved**: All code comments now in English
- **Improved**: Each file kept under 300 lines for better maintainability
- **Fixed**: Go version from invalid `1.25.3` to valid `1.23.0`
- **Changed** (behavioral, not just internal): removed the hardcoded
  `created_at DESC` default ordering that used to apply whenever no valid
  order was requested. Callers relying on that implicit default without
  calling `WithDefaultOrder` see a behavior change — see the note below.

### 🛡️ Security

- Added regex-based column name validation (`^[a-zA-Z0-9_.]+$`)
- All column names validated before SQL query construction
- Prevents SQL injection via column name manipulation
- Validates both searchable and orderable column mappings

### 🔄 Backward Compatibility

Function and method signatures are unchanged from the pre-refactor version,
and the new column-name validation is transparent for callers already
using valid identifiers. However, **default behavior is not fully
unchanged**: this release removed the previously hardcoded `created_at
DESC` fallback (see the Fixed section above). Code that depended on that
implicit default without calling `WithDefaultOrder` will now see queries
run with no `ORDER BY` when the requested order is invalid — call
`WithDefaultOrder` explicitly to restore a deterministic default.
(v2.0.0 closes this gap further by appending a primary-key tie-breaker
automatically; see the v2.0.0 entry above.)

### 📦 Dependencies

- Go 1.23.0 or higher
- `github.com/gin-gonic/gin` v1.11.0
- `gorm.io/gorm` v1.31.0

---

## Migration Guide

### Existing Users

Function and method signatures are unchanged. See the Backward Compatibility
note above: if your code relied on the implicit `created_at DESC` fallback,
call `WithDefaultOrder` explicitly to keep a deterministic order.

### Recommended Updates

1. **Add default order configuration** (optional but recommended):
   ```go
   opts := datatables.NewOptions().
       WithDefaultOrder("id DESC")
   ```

2. **Use error helper** (optional):
   ```go
   if err != nil {
       datatables.JSONError(c, 500, err.Error())
       return
   }
   ```

### New Users

See `examples/basic_usage.go` for complete examples.

---

## What's Next?

Potential future enhancements (feedback welcome):
- [x] Support for individual column search — shipped in v2.1.0
- [ ] Advanced filtering options
- [ ] Export functionality (CSV, Excel)
- [ ] Caching layer for frequent queries
- [ ] Middleware for automatic setup

---

## Links

- [README](README.md) - Full documentation
- [MIGRATING.md](MIGRATING.md) - v1 → v2 migration guide
- [Examples](examples/)
- [Report Issues](https://github.com/bonarizki/gin-datatables/issues)
