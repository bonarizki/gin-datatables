# Migration Guide: v1 → v2

v2.0.0 fixes a cross-tenant data leak in global search, restores
spec-compliant ordering, bounds pagination, and fixes the response and
struct-conversion output contract. Several of those fixes required breaking
API changes — see [CHANGELOG.md](CHANGELOG.md) for the complete list and the
reasoning behind each one. This guide is the step-by-step upgrade path.

There is no compatibility shim: v1's behavior in the areas below (unbounded
search, non-spec ordering, unbounded pagination, nested/dropped embedded
struct fields, the wrapped JSON response) is not preserved as an opt-in,
because each of those behaviors was the bug being fixed.

---

## 1. Update the import path

Go requires the major version in the module path for v2+.

```diff
-go get github.com/bonarizki-dat/Datatables-Gin
+go get github.com/bonarizki/gin-datatables/v2
```

```diff
-import "github.com/bonarizki-dat/Datatables-Gin/datatables"
+import "github.com/bonarizki/gin-datatables/v2/datatables"
```

## 2. Update `WithDefaultOrder` calls

The free-form SQL string is gone. Split it into a validated column and
direction:

```diff
-opts := datatables.NewOptions().WithDefaultOrder("created_at DESC")
+opts := datatables.NewOptions().WithDefaultOrder("created_at", "desc")
```

For a multi-column default order, call it more than once — clauses apply in
call order:

```go
opts := datatables.NewOptions().
    WithDefaultOrder("status", "asc").
    WithDefaultOrder("created_at", "desc")
```

`col` and `dir` are now validated the same way as `searchable`/`orderable`
columns. `OfReturn` returns `ErrInvalidColumnName` or
`ErrInvalidOrderDirection` if either check fails, instead of concatenating
unvalidated SQL into `ORDER BY`.

## 3. If you call `ParseParams` directly

Most callers only use `OfReturn`, which handles this internally. If you call
`ParseParams` yourself, it now takes `Options` (needed to read the page-size
bounds) and returns `Orders []dto.Order` instead of `Order`/`Dir` strings:

```diff
-params := datatables.ParseParams(c)
-if params.Order != "" { ... params.Dir ... }
+params := datatables.ParseParams(c, opts)
+for _, o := range params.Orders { ... o.Column ... o.Dir ... }
```

## 4. Re-check any code relying on unbounded `length`

`length=0`, an unparseable `length`, or `length=-1` used to disable
pagination entirely, returning every row. In v2 these all fall back to
`Options.DefaultPageSize` (10 by default). If you intentionally relied on
`length=-1` meaning "all rows", opt back in explicitly:

```go
opts := datatables.NewOptions().WithUnlimitedPageSize(true)
```

Without this option, a client sending `length=-1` now gets
`DefaultPageSize` rows instead of the whole table.

## 5. Check for a relied-upon implicit sort order

If your endpoint never called `WithDefaultOrder` and depended on rows
happening to come back in a particular order when the requested sort was
invalid or absent, that was never guaranteed even in v1 — the database was
free to return any order once no `ORDER BY` was present. v2 always appends a
primary-key tie-breaker, so paging is stable, but if you need a specific
*default* sort (not just a stable one), add it explicitly:

```go
opts := datatables.NewOptions().WithDefaultOrder("id", "desc")
```

## 6. Check embedded structs in your models

If your model embeds `gorm.Model` or any other struct, v1 either dropped
those fields from the response entirely (unexported embed) or nested them
under the embedded type's name instead of the field's own JSON key (exported
embed, e.g. `{"Model": {"ID": 7, ...}}` instead of `{"id": 7, ...}`). v2
flattens embedded fields the same way `encoding/json` does. If your frontend
was built around the v1 shape (or around the field being silently absent),
update it to read the field at the root — this is very likely a bug fix for
you, not a change you need to code around, but it does change the response
body.

## 7. Check any code that inspects the raw JSON response

`datatables.JSON()` now writes `{draw, recordsTotal, recordsFiltered, data}`
directly at the response root instead of wrapping it as
`{success, message, data: {...}, errors: null}`. If you had frontend code
reaching into `response.data.data` to work around the old shape, remove that
workaround — `response.data` is now the array of rows directly, matching
what a stock DataTables client expects.

`datatables.JSONError()` now writes `{"error": "..."}` instead of the old
`SuccessResponse` shape. Update any client-side error handling that read
`response.errors` or `response.message` on failure.

## 8. Stop passing raw error messages to clients

The bundled examples used to call `datatables.JSONError(c, 500,
err.Error())`. Don't do this — `err.Error()` can contain database and
schema details. Log the error server-side and send a fixed, safe message:

```go
result, err := datatables.OfReturn(c, query, &users, searchable, orderable, opts)
if err != nil {
    log.Printf("failed to load users: %v", err)
    datatables.JSONError(c, http.StatusInternalServerError, "failed to load users")
    return
}
```

## 9. Optional: adopt `WithStrictOrder` and `errors.Is`

New in v2, not required for migration, but worth adding:

```go
opts := datatables.NewOptions().WithStrictOrder(true)
```

This turns a mismatched frontend/backend order key into
`ErrInvalidOrderColumn` immediately, instead of a silently ignored sort.
Combined with the new `Unwrap()` support, you can branch on error kind:

```go
if errors.Is(err, datatables.ErrInvalidOrderColumn) {
    // a frontend/backend wiring mistake, not a database failure
}
```

---

## Quick reference

| Area | v1 | v2 |
| --- | --- | --- |
| Module path | `github.com/bonarizki-dat/Datatables-Gin` | `github.com/bonarizki/gin-datatables/v2` |
| `ParseParams` | `ParseParams(c)` | `ParseParams(c, opts)` |
| Order in `dto.Params` | `Order string`, `Dir string` | `Orders []dto.Order` |
| `WithDefaultOrder` | `WithDefaultOrder("id DESC")` | `WithDefaultOrder("id", "desc")` |
| `Options.DefaultOrder` | `string` | `DefaultOrders []dto.Order` |
| Unbounded `length` | `0`/invalid/`-1` disabled `LIMIT` | Falls back to `DefaultPageSize`; `-1` needs `WithUnlimitedPageSize(true)` |
| Embedded structs | Dropped or nested under the type name | Flattened, outer field wins on collision |
| `JSON()` body | `{success, message, data: {...}}` | `{draw, recordsTotal, recordsFiltered, data}` at root |
| `JSONError()` body | `{success:false, message, data:nil, errors}` | `{"error": "..."}` |
| `AddColumns`/`EditColumns` callback arg | Original row, missing the index column | Row as built so far, including the index column |
| `ErrColumnNotFound`, `ErrDefaultOrderColumn` | Declared, never returned | Removed |
| `dto.SuccessResponse`, `dto.ResponseDatatables` | Used by `JSON()`/`JSONError()` | Removed |
