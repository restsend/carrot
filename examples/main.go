package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/restsend/carrot"
	"gorm.io/gorm"
)

type Product struct {
	UUID      string    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Enabled   bool      `json:"enabled"`
}

func main() {
	db, _ := carrot.InitDatabase(nil, "", "")

	r := gin.Default()
	if err := carrot.InitCarrot(db, r); err != nil {
		panic(err)
	}

	// Check Default Value
	// carrot.CheckValue(db, carrot.KEY_SITE_NAME, "Your Name")

	// Connect user event, eg. Login, Create
	carrot.Sig().Connect(carrot.SigUserCreate, func(sender any, params ...any) {
		user := sender.(*carrot.User)
		log.Println("create user", user.GetVisibleName())
	})
	carrot.Sig().Connect(carrot.SigUserLogin, func(sender any, params ...any) {
		user := sender.(*carrot.User)
		log.Println("user logined", user.GetVisibleName())
	})

	// Register WebObject
	RegisterWebObjectHandler(r, db)

	// Visit:
	//  http://localhost:8080/
	//  http://localhost:8080/auth/login
	r.Run(":8080")
}

// Check example.http
func RegisterWebObjectHandler(r *gin.Engine, db *gorm.DB) {
	product := carrot.WebObject[Product]{
		Model:     Product{},
		Searchs:   []string{"Name"},
		Editables: []string{"Name", "Enabled"},
		Filters:   []string{"Name", "CreatedAt", "Enabled"},
		GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return db
		},
		// You can Specify how the id is generated.
		Init: func(ctx *gin.Context, v *Product) {
			v.UUID = carrot.RandText(10)
		},
	}

	if err := product.RegisterObject(r); err != nil {
		panic(err)
	}

	if err := carrot.MakeMigrates(db, []any{Product{}}); err != nil {
		panic(err)
	}
}
