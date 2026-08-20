package datatables

// applyOptions processes DataTables customization options such as adding new columns,
// editing existing ones, removing unwanted fields, and setting row indexes.
//
// The transformation is applied in the following order:
//  1. Copy original row data
//  2. Add index column (DT_RowIndex)
//  3. Add custom columns (from Options.AddColumns)
//  4. Edit existing columns (from Options.EditColumns)
//  5. Remove unwanted columns (from Options.RemoveColumns)
//
// Parameters:
//   - data: Slice of maps representing rows
//   - opts: Options struct containing transformation rules
//   - start: Starting offset for index calculation (used when ResetIndex is false)
//
// Returns the transformed data with all options applied.
func applyOptions(data []map[string]interface{}, opts Options, start int) []map[string]interface{} {
	if data == nil {
		return nil
	}

	out := make([]map[string]interface{}, 0, len(data))

	for i, row := range data {
		// Create a new map to avoid modifying the original
		newRow := make(map[string]interface{})
		for k, v := range row {
			newRow[k] = v
		}

		// Step 1: Add index column
		if opts.IndexColumn != "" {
			if opts.ResetIndex {
				// Index starts from 1 on each page
				newRow[opts.IndexColumn] = i + 1
			} else {
				// Index continues from previous pages
				newRow[opts.IndexColumn] = start + i + 1
			}
		}

		// Step 2: Add custom columns. Callbacks receive newRow (not the
		// original row), so they can read the index column added in Step 1.
		// Map iteration order is unspecified, so an AddColumns callback
		// should not depend on another AddColumns entry also being present.
		for colName, fn := range opts.AddColumns {
			newRow[colName] = fn(newRow)
		}

		// Step 3: Edit existing columns. Callbacks also see newRow, so they
		// observe columns added in Step 1/2 as well as the original data.
		for colName, fn := range opts.EditColumns {
			if val, ok := newRow[colName]; ok {
				newRow[colName] = fn(val, newRow)
			}
		}

		// Step 4: Remove unwanted columns
		for _, col := range opts.RemoveColumns {
			delete(newRow, col)
		}

		out = append(out, newRow)
	}

	return out
}
