package datatables_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/bonarizki/gin-datatables/v2/datatables"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type exampleUser struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func ExampleOfReturn() {
	gin.SetMode(gin.TestMode)
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	_ = db.AutoMigrate(&exampleUser{})
	db.Create(&[]exampleUser{
		{Name: "Ada", Email: "ADA@EXAMPLE.COM", Password: "x"},
		{Name: "Linus", Email: "linus@example.com", Password: "y"},
	})

	r := gin.New()
	r.GET("/users", func(c *gin.Context) {
		var users []exampleUser
		result, err := datatables.OfReturn(
			c,
			db.Model(&exampleUser{}),
			&users,
			[]string{"name", "email"},
			map[string]string{"name": "name", "email": "email"},
			datatables.NewOptions().
				WithDefaultOrder("name", "asc").
				WithColumnSearch(map[string]string{"name": "name"}).
				Edit("email", func(value any, _ map[string]any) any {
					return strings.ToLower(fmt.Sprint(value))
				}).
				Remove("password"),
		)
		if err != nil {
			datatables.JSONError(c, http.StatusInternalServerError, "failed to load data")
			return
		}
		datatables.JSON(c, result)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users?draw=1&start=0&length=10&search[value]=ada", nil))
	fmt.Println(w.Body.String())
	// Output:
	// {"draw":1,"recordsTotal":2,"recordsFiltered":1,"data":[{"DT_RowIndex":1,"email":"ada@example.com","id":1,"name":"Ada"}]}
}
