package examples

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/bonarizki-dat/Datatables-Gin/v2/datatables"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// User model example
type User struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// BasicUsage demonstrates the simplest way to use the package.
func BasicUsage(c *gin.Context, db *gorm.DB) {
	var users []User

	// Define searchable columns
	searchable := []string{"name", "email"}

	// Map frontend column names to database columns
	orderable := map[string]string{
		"name":  "name",
		"email": "email",
	}

	opts := datatables.NewOptions()

	// Process DataTables request
	result, err := datatables.OfReturn(
		c,
		db.Model(&User{}),
		&users,
		searchable,
		orderable,
		opts,
	)

	if err != nil {
		// Log the real error server-side; never send err.Error() to the
		// client, since it can contain database/schema details.
		log.Printf("BasicUsage: OfReturn failed: %v", err)
		datatables.JSONError(c, http.StatusInternalServerError, "failed to load users")
		return
	}

	datatables.JSON(c, result)
}

// ImprovedUsage shows the default-order and strict-order options.
func ImprovedUsage(c *gin.Context, db *gorm.DB) {
	var users []User

	searchable := []string{"name", "email"}

	orderable := map[string]string{
		"name":    "name",
		"email":   "email",
		"created": "created_at",
	}

	opts := datatables.NewOptions().
		WithDefaultOrder("created_at", "desc"). // applied when the request has no valid order
		WithStrictOrder(true).                  // fail loudly on an unknown order key instead of silently skipping it
		WithIndex("row_num", true)              // custom index column

	result, err := datatables.OfReturn(
		c,
		db.Model(&User{}),
		&users,
		searchable,
		orderable,
		opts,
	)

	if err != nil {
		log.Printf("ImprovedUsage: OfReturn failed: %v", err)
		datatables.JSONError(c, http.StatusInternalServerError, "failed to load users")
		return
	}

	datatables.JSON(c, result)
}

// AdvancedUsage shows all features including custom columns.
func AdvancedUsage(c *gin.Context, db *gorm.DB) {
	var users []User

	searchable := []string{"name", "email"}

	orderable := map[string]string{
		"name":    "name",
		"email":   "email",
		"created": "created_at",
	}

	opts := datatables.NewOptions().
		WithDefaultOrder("created_at", "desc").
		WithIndex("DT_RowIndex", false).
		Add("actions", func(row map[string]interface{}) interface{} {
			// User.ID is uint, so it comes through as a numeric type, not a
			// string; format it instead of asserting row["id"].(string).
			return fmt.Sprintf(`<a href="/users/%v/edit">Edit</a>`, row["id"])
		}).
		Remove("created_at") // Hide internal field

	result, err := datatables.OfReturn(
		c,
		db.Model(&User{}),
		&users,
		searchable,
		orderable,
		opts,
	)

	if err != nil {
		log.Printf("AdvancedUsage: OfReturn failed: %v", err)
		datatables.JSONError(c, http.StatusInternalServerError, "failed to load users")
		return
	}

	datatables.JSON(c, result)
}
