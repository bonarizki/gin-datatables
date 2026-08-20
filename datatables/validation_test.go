package datatables

import (
	"errors"
	"testing"

	"github.com/bonarizki-dat/Datatables-Gin/v2/datatables/dto"
)

func TestIsValidColumnName(t *testing.T) {
	tests := []struct {
		name     string
		column   string
		expected bool
	}{
		{"Valid simple column", "user_id", true},
		{"Valid column with table", "users.id", true},
		{"Valid mixed case", "UserName", true},
		{"Valid with numbers", "field123", true},
		{"Empty string", "", false},
		{"SQL injection attempt - semicolon", "id; DROP TABLE users", false},
		{"SQL injection attempt - comment", "id--", false},
		{"SQL injection attempt - quote", "id'", false},
		{"Special characters", "id@#$", false},
		{"Spaces", "user name", false},
		{"Valid complex", "table1.column_name_2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidColumnName(tt.column)
			if result != tt.expected {
				t.Errorf("isValidColumnName(%q) = %v, want %v", tt.column, result, tt.expected)
			}
		})
	}
}

func TestValidateSearchableColumns(t *testing.T) {
	tests := []struct {
		name      string
		columns   []string
		shouldErr bool
	}{
		{
			name:      "Valid columns",
			columns:   []string{"name", "email", "users.id"},
			shouldErr: false,
		},
		{
			name:      "Empty list",
			columns:   []string{},
			shouldErr: false,
		},
		{
			name:      "Invalid column with special chars",
			columns:   []string{"name", "email'; DROP TABLE users--"},
			shouldErr: true,
		},
		{
			name:      "Invalid column with space",
			columns:   []string{"user name"},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSearchableColumns(tt.columns)
			if (err != nil) != tt.shouldErr {
				t.Errorf("validateSearchableColumns() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestValidateOrderableColumns(t *testing.T) {
	tests := []struct {
		name      string
		columns   map[string]string
		shouldErr bool
	}{
		{
			name: "Valid columns",
			columns: map[string]string{
				"name":    "users.name",
				"email":   "users.email",
				"created": "created_at",
			},
			shouldErr: false,
		},
		{
			name:      "Empty map",
			columns:   map[string]string{},
			shouldErr: false,
		},
		{
			name: "Invalid key",
			columns: map[string]string{
				"name; DROP TABLE": "users.name",
			},
			shouldErr: true,
		},
		{
			name: "Invalid value",
			columns: map[string]string{
				"name": "users.name'; DROP TABLE users--",
			},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOrderableColumns(tt.columns)
			if (err != nil) != tt.shouldErr {
				t.Errorf("validateOrderableColumns() error = %v, shouldErr %v", err, tt.shouldErr)
			}
		})
	}
}

func TestValidateDefaultOrders(t *testing.T) {
	tests := []struct {
		name      string
		orders    []dto.Order
		shouldErr bool
		wantErr   error
	}{
		{
			name:      "Valid single order",
			orders:    []dto.Order{{Column: "created_at", Dir: "desc"}},
			shouldErr: false,
		},
		{
			name:      "Valid multi-column order",
			orders:    []dto.Order{{Column: "status", Dir: "asc"}, {Column: "created_at", Dir: "desc"}},
			shouldErr: false,
		},
		{
			name:      "Nil orders",
			orders:    nil,
			shouldErr: false,
		},
		{
			name:      "Invalid column",
			orders:    []dto.Order{{Column: "id; DROP TABLE users", Dir: "asc"}},
			shouldErr: true,
			wantErr:   ErrInvalidColumnName,
		},
		{
			name:      "Invalid direction",
			orders:    []dto.Order{{Column: "created_at", Dir: "up"}},
			shouldErr: true,
			wantErr:   ErrInvalidOrderDirection,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDefaultOrders(tt.orders)
			if (err != nil) != tt.shouldErr {
				t.Errorf("validateDefaultOrders() error = %v, shouldErr %v", err, tt.shouldErr)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("validateDefaultOrders() error = %v, want errors.Is match for %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidationErrorUnwrap(t *testing.T) {
	err := validateSearchableColumns([]string{"bad; name"})
	if !errors.Is(err, ErrInvalidColumnName) {
		t.Errorf("expected errors.Is(err, ErrInvalidColumnName) to be true, got err=%v", err)
	}
}
