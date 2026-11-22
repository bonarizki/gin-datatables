package datatables_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bonarizki/gin-datatables/v2/datatables"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type dtTestRow struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:64"`
}

func TestOfReturn_SQLite_PaginationAndSearch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	if err := db.AutoMigrate(&dtTestRow{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rows := []dtTestRow{{Name: "alpha"}, {Name: "beta"}, {Name: "alphabet"}}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?draw=1&start=0&length=2&search[value]=alpha", nil)

	var out []dtTestRow
	query := db.Model(&dtTestRow{})
	result, err := datatables.OfReturn(
		c,
		query,
		&out,
		[]string{"name"},
		map[string]string{"name": "name"},
		datatables.NewOptions(),
	)
	if err != nil {
		t.Fatalf("OfReturn: %v", err)
	}
	if result.RecordsFiltered != 2 {
		t.Fatalf("RecordsFiltered = %d, want 2", result.RecordsFiltered)
	}
	switch data := result.Data.(type) {
	case []map[string]any:
		if len(data) != 2 {
			t.Fatalf("page size = %d, want 2", len(data))
		}
	default:
		t.Fatalf("unexpected Data type %T", result.Data)
	}
}