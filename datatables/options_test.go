package datatables

import (
	"strings"
	"testing"
)

func TestNewOptions(t *testing.T) {
	opts := NewOptions()

	if opts.IndexColumn != "DT_RowIndex" {
		t.Errorf("Expected IndexColumn='DT_RowIndex', got %q", opts.IndexColumn)
	}
	if opts.ResetIndex != false {
		t.Error("Expected ResetIndex=false")
	}
	if opts.DefaultOrders != nil {
		t.Errorf("Expected DefaultOrders=nil, got %v", opts.DefaultOrders)
	}
	if opts.StrictOrder != false {
		t.Error("Expected StrictOrder=false")
	}
	if opts.DefaultPageSize != 10 {
		t.Errorf("Expected DefaultPageSize=10, got %d", opts.DefaultPageSize)
	}
	if opts.MaxPageSize != 500 {
		t.Errorf("Expected MaxPageSize=500, got %d", opts.MaxPageSize)
	}
	if opts.UnlimitedPageSize != false {
		t.Error("Expected UnlimitedPageSize=false")
	}
	if opts.AddColumns == nil {
		t.Error("AddColumns should be initialized")
	}
	if opts.EditColumns == nil {
		t.Error("EditColumns should be initialized")
	}
	if opts.RemoveColumns == nil {
		t.Error("RemoveColumns should be initialized")
	}
}

func TestOptionsWithIndex(t *testing.T) {
	opts := NewOptions().WithIndex("row_num", true)

	if opts.IndexColumn != "row_num" {
		t.Errorf("Expected IndexColumn='row_num', got %q", opts.IndexColumn)
	}
	if opts.ResetIndex != true {
		t.Error("Expected ResetIndex=true")
	}
}

func TestOptionsWithDefaultOrder(t *testing.T) {
	opts := NewOptions().WithDefaultOrder("created_at", "desc")

	if len(opts.DefaultOrders) != 1 {
		t.Fatalf("Expected 1 default order, got %d", len(opts.DefaultOrders))
	}
	if opts.DefaultOrders[0].Column != "created_at" || opts.DefaultOrders[0].Dir != "desc" {
		t.Errorf("Expected {created_at desc}, got %+v", opts.DefaultOrders[0])
	}
}

func TestOptionsWithDefaultOrderMultiColumn(t *testing.T) {
	opts := NewOptions().WithDefaultOrder("status", "asc").WithDefaultOrder("created_at", "desc")

	if len(opts.DefaultOrders) != 2 {
		t.Fatalf("Expected 2 default orders, got %d", len(opts.DefaultOrders))
	}
	if opts.DefaultOrders[0].Column != "status" || opts.DefaultOrders[1].Column != "created_at" {
		t.Errorf("Expected default orders in call order, got %+v", opts.DefaultOrders)
	}
}

func TestOptionsWithStrictOrder(t *testing.T) {
	opts := NewOptions().WithStrictOrder(true)
	if !opts.StrictOrder {
		t.Error("Expected StrictOrder=true")
	}
}

func TestOptionsWithPageSize(t *testing.T) {
	opts := NewOptions().WithPageSize(25, 200)
	if opts.DefaultPageSize != 25 || opts.MaxPageSize != 200 {
		t.Errorf("Expected {25 200}, got {%d %d}", opts.DefaultPageSize, opts.MaxPageSize)
	}
}

func TestOptionsWithUnlimitedPageSize(t *testing.T) {
	opts := NewOptions().WithUnlimitedPageSize(true)
	if !opts.UnlimitedPageSize {
		t.Error("Expected UnlimitedPageSize=true")
	}
}

func TestOptionsAdd(t *testing.T) {
	opts := NewOptions().Add("full_name", func(row map[string]interface{}) interface{} {
		return row["first_name"].(string) + " " + row["last_name"].(string)
	})

	if len(opts.AddColumns) != 1 {
		t.Errorf("Expected 1 custom column, got %d", len(opts.AddColumns))
	}

	if _, exists := opts.AddColumns["full_name"]; !exists {
		t.Error("full_name column should exist in AddColumns")
	}

	// Test the function
	row := map[string]interface{}{
		"first_name": "John",
		"last_name":  "Doe",
	}
	result := opts.AddColumns["full_name"](row)
	if result != "John Doe" {
		t.Errorf("Expected 'John Doe', got %v", result)
	}
}

func TestOptionsEdit(t *testing.T) {
	opts := NewOptions().Edit("email", func(value interface{}, row map[string]interface{}) interface{} {
		return strings.ToLower(value.(string))
	})

	if len(opts.EditColumns) != 1 {
		t.Errorf("Expected 1 edit column, got %d", len(opts.EditColumns))
	}

	if _, exists := opts.EditColumns["email"]; !exists {
		t.Error("email column should exist in EditColumns")
	}

	// Test the function
	row := map[string]interface{}{"email": "TEST@EXAMPLE.COM"}
	result := opts.EditColumns["email"]("TEST@EXAMPLE.COM", row)
	if result != "test@example.com" {
		t.Errorf("Expected 'test@example.com', got %v", result)
	}
}

func TestOptionsRemove(t *testing.T) {
	opts := NewOptions().Remove("password", "internal_id")

	if len(opts.RemoveColumns) != 2 {
		t.Errorf("Expected 2 columns to remove, got %d", len(opts.RemoveColumns))
	}

	expected := []string{"password", "internal_id"}
	for i, col := range expected {
		if opts.RemoveColumns[i] != col {
			t.Errorf("Expected RemoveColumns[%d]=%q, got %q", i, col, opts.RemoveColumns[i])
		}
	}
}

func TestOptionsChaining(t *testing.T) {
	opts := NewOptions().
		WithIndex("row_num", true).
		WithDefaultOrder("id", "desc").
		Add("test_col", func(row map[string]interface{}) interface{} {
			return "test"
		}).
		Edit("name", func(value interface{}, row map[string]interface{}) interface{} {
			return strings.ToUpper(value.(string))
		}).
		Remove("secret")

	if opts.IndexColumn != "row_num" {
		t.Error("Chaining failed for WithIndex")
	}
	if len(opts.DefaultOrders) != 1 || opts.DefaultOrders[0].Column != "id" {
		t.Error("Chaining failed for WithDefaultOrder")
	}
	if len(opts.AddColumns) != 1 {
		t.Error("Chaining failed for Add")
	}
	if len(opts.EditColumns) != 1 {
		t.Error("Chaining failed for Edit")
	}
	if len(opts.RemoveColumns) != 1 {
		t.Error("Chaining failed for Remove")
	}
}
