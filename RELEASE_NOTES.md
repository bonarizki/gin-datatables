# Release Notes — v2.0.0

## Summary

This release fixes a cross-tenant data leak in global search, restores
spec-compliant DataTables ordering, bounds pagination, and fixes the
response/converter output contract. It is a breaking change — see
[MIGRATING.md](MIGRATING.md) for the upgrade path and
[CHANGELOG.md](CHANGELOG.md) for the full list of changes.

### Highlights

- 🔴 **Security fix**: global search could bypass a caller's own `WHERE`
  clause (e.g. tenant isolation) because the OR chain across searchable
  columns wasn't grouped. Fixed, with a regression test.
- 🔴 **Security fix**: `length=0`/invalid/`-1` disabled `LIMIT` entirely,
  letting a client dump an entire table with one query parameter. `length`
  is now always bounded.
- 🔴 **Correctness fix**: `order[0][column]` was read as a column name
  instead of the numeric index the DataTables spec sends, so a stock
  client's sort was silently dropped.
- 🔴 **Correctness fix**: with no valid order, queries ran with no `ORDER
  BY` while `LIMIT`/`OFFSET` was still applied — paging could duplicate or
  skip rows. A primary-key tie-breaker is now always appended.
- 🔴 **Output fix**: embedded struct fields (e.g. `gorm.Model`) were dropped
  or nested instead of flattened, and `datatables.JSON()` wrapped the
  DataTables payload in an envelope the stock client can't read. Both are
  fixed.
- ✅ Test coverage for `datatables/parser.go` and `datatables/processor.go`
  went from 0% to fully covered by new SQL-level tests; package coverage
  is 92.8% (measured via `go test ./datatables/... -cover`).

---

## Is this backward compatible?

**No.** This is a breaking release; the module path changes to
`github.com/bonarizki/gin-datatables/v2` and several APIs changed shape
because the old shape *was* the bug. See [MIGRATING.md](MIGRATING.md) for a
step-by-step guide — most call sites need a handful of mechanical changes
(import path, `WithDefaultOrder` argument split, response body shape on the
frontend side).

### Example: what changes for a typical caller

```go
// v1
import "github.com/bonarizki-dat/Datatables-Gin/datatables"

opts := datatables.NewOptions().WithDefaultOrder("created_at DESC")
result, err := datatables.OfReturn(c, db.Model(&User{}), &users, searchable, orderable, opts)
if err != nil {
    c.JSON(500, gin.H{"error": err.Error()})
    return
}
datatables.JSON(c, result)
```

```go
// v2
import "github.com/bonarizki/gin-datatables/v2/datatables"

opts := datatables.NewOptions().WithDefaultOrder("created_at", "desc")
result, err := datatables.OfReturn(c, db.Model(&User{}), &users, searchable, orderable, opts)
if err != nil {
    log.Printf("OfReturn failed: %v", err) // don't send err.Error() to the client
    datatables.JSONError(c, http.StatusInternalServerError, "failed to load users")
    return
}
datatables.JSON(c, result) // now writes {draw, recordsTotal, recordsFiltered, data} at the root
```

---

## Security details

### The search data leak (fixed)

Given a caller-scoped query and a search term matching the second
searchable column:

```go
// db.Model(&User{}).Where("tenant_id = ?", "A")
// searchable: []string{"name", "code"}, search value: "x"
```

v1 generated:

```sql
WHERE tenant_id = 'A' AND LOWER(name) LIKE LOWER('%x%') OR LOWER(code) LIKE LOWER('%x%')
```

`AND` binds tighter than `OR`, so any row matching `code` regardless of
`tenant_id` was included in the result. v2 generates:

```sql
WHERE tenant_id = 'A' AND (LOWER(name) LIKE LOWER('%x%') ESCAPE '\' OR LOWER(code) LIKE LOWER('%x%') ESCAPE '\')
```

Search values are also escaped for `%`, `_`, and `\`, so a search for
`100%` no longer matches every row.

### Unbounded pagination (fixed)

`length=0`, `length=abc`, and `length=-1` all disabled `LIMIT` in v1, so a
single query parameter could force a full table dump. In v2, `length` is
always resolved to a positive value bounded by `Options.DefaultPageSize`
(10) and `Options.MaxPageSize` (500), unless the server explicitly opts
into `-1` meaning "all rows" via `Options.WithUnlimitedPageSize(true)`.

---

## Test coverage

```bash
$ go test ./datatables/... -cover
PASS
coverage: 92.8% of statements
```

New in this release: `parser_test.go` (table-driven `ParseParams` tests) and
`processor_test.go` (SQL assertions via `gorm.Config{DryRun: true}` for
search grouping, LIKE escaping, ordering, the tie-breaker, `StrictOrder`,
and an end-to-end `OfReturn` smoke test) — the two files that previously had
no tests at all, and where every fix in this release lives.

---

## Upgrading

```bash
go get -u github.com/bonarizki/gin-datatables/v2
```

Then update import paths and follow [MIGRATING.md](MIGRATING.md).

### FAQ

**Do I need to change my code?**
Yes — at minimum the import path and any `WithDefaultOrder` calls. See
[MIGRATING.md](MIGRATING.md) for the full checklist.

**Will this change my API's response body?**
Yes, if you use `datatables.JSON()`/`JSONError()`: the response shape now
matches what a stock DataTables client expects at the root, instead of
being wrapped in a `{success, message, data}` envelope. If your frontend
had a workaround for the old shape, remove it.

**Will this change query behavior even if I don't touch my code?**
Only where v1's behavior was the bug: an endpoint that relied on
`length=-1`/`length=0` returning all rows now needs
`WithUnlimitedPageSize(true)`; an endpoint whose `WithDefaultOrder` string
argument doesn't split cleanly into `(column, direction)` needs updating.

**Where do the sentinel errors (`ErrInvalidColumnName`, etc.) come from now?**
`*ValidationError` implements `Unwrap()` in v2, so `errors.Is(err,
datatables.ErrInvalidColumnName)` works. In v1 this never matched, even
though the sentinels were declared.
