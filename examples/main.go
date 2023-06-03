package main

import (
	"errors"
	"flag"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/restsend/carrot"
	"gorm.io/gorm"
)

type Product struct {
	UUID      string    `json:"id" gorm:"primarykey"`
	GroupID   int       `json:"-"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Enabled   bool      `json:"enabled"`
}

type User struct {
	ID        uint       `json:"id" gorm:"primarykey"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	Name      string     `json:"name"`
	Age       int        `json:"age"`
	Enabled   bool       `json:"enabled"`
	LastLogin *time.Time `json:"lastLogin,omitempty"`
}

func main() {
	var superUserEmail string
	var superUserPassword string

	flag.StringVar(&superUserEmail, "superuser", "", "Create an super user with email")
	flag.StringVar(&superUserPassword, "password", "", "Super user password")
	flag.Parse()

	db, _ := carrot.InitDatabase(nil, "", "")

	if superUserEmail != "" && superUserPassword != "" {
		u, err := carrot.GetUserByEmail(db, superUserEmail)
		if err == nil && u != nil {
			carrot.SetPassword(db, u, superUserPassword)
			carrot.Warning("Update super with new password")
		} else {
			u, err = carrot.CreateUser(db, superUserEmail, superUserPassword)
			if err != nil {
				panic(err)
			}
		}
		u.IsStaff = true
		u.Actived = true
		u.Enabled = true
		u.IsSuperUser = true
		db.Save(u)
		carrot.Warning("Create super user:", superUserEmail)
		return
	}

	r := gin.Default()
	if err := carrot.InitCarrot(db, r); err != nil {
		panic(err)
	}

	as, ok := r.HTMLRender.(*carrot.StaticAssets)
	if ok {
		paths := []string{carrot.HintAssetsRoot([]string{"./", "../"})}
		as.Paths = append(paths, as.Paths...)
	}

	// Check Default Value
	carrot.CheckValue(db, carrot.KEY_SITE_NAME, "Carrot")

	// Connect user event, eg. Login, Create
	carrot.Sig().Connect(carrot.SigUserCreate, func(sender any, params ...any) {
		user := sender.(*carrot.User)
		carrot.Info("create user: ", user.GetVisibleName())
	})
	carrot.Sig().Connect(carrot.SigUserLogin, func(sender any, params ...any) {
		user := sender.(*carrot.User)
		carrot.Info("user logined: ", user.GetVisibleName())
	})

	objs := GetWebObjects(db)
	carrot.RegisterObjects(r.Group("/"), objs)

	// Register Admin
	/*
		quick start:
		DSN=file:demo.db go run .

		1. Create a super user
			go run . -superuser ADMIN@YOUR -password XXXXX
		2. Login with super user
			http://localhost:8080/admin
	*/
	adminobjs := carrot.GetCarrotAdminObjects()
	carrot.RegisterAdmins(r.Group("/admin"), db, adminobjs)
	r.Run(":8080")
}

func GetWebObjects(db *gorm.DB) []carrot.WebObject {
	return []carrot.WebObject{
		// Basic Demo
		// Check API File: user.http
		// PUT 		http://localhost:8890/user
		// GET 		http://localhost:8890/user/:key
		// PATCH	http://localhost:8890/user/:key
		// POST 	http://localhost:8890/user
		// DELETE http://localhost:8890/user/:key
		// DELETE http://localhost:8890/user
		{
			Name:        "user",
			Model:       &User{},
			Searchables: []string{"Name", "Enabled"},
			Editables:   []string{"Name", "Age", "Enabled"},
			Filterables: []string{"Name", "CreatedAt", "Age", "Enabled"},
			Orderables:  []string{"CreatedAt", "Age", "Enabled"},
			GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
				return db
			},
		},
		// Advanced Demo
		// Check API File: product.http
		// PUT 		http://localhost:8890/product
		// GET 		http://localhost:8890/product/:key
		// PATCH	http://localhost:8890/product/:key
		// POST 	http://localhost:8890/product
		// DELETE http://localhost:8890/product/:key
		// DELETE http://localhost:8890/product
		{
			Name:        "product",
			Model:       &Product{},
			Searchables: []string{"Name"},
			Editables:   []string{"Name", "Enabled"},
			Filterables: []string{"Name", "CreatedAt", "Enabled"},
			Orderables:  []string{"CreatedAt"},
			GetDB: func(c *gin.Context, isCreate bool) *gorm.DB {
				return db
			},
			BeforeCreate: func(ctx *gin.Context, vptr any) error {
				p := (vptr).(*Product)
				p.UUID = carrot.RandText(8)
				p.GroupID = rand.Intn(5)
				return nil
			},
			BeforeDelete: func(ctx *gin.Context, vptr any) error {
				p := (vptr).(*Product)
				if p.Enabled {
					return errors.New("product is enabled, can not delete")
				}
				return nil
			},
			// Custom Query View
			// GET http://localhost:8890/product/all_enabled
			Views: []carrot.QueryView{
				{
					Name:   "all_enabled",
					Method: "GET",
					Prepare: func(db *gorm.DB, c *gin.Context) (*gorm.DB, *carrot.QueryForm, error) {
						// SELECT (id, name) FROM products WHERE enabled = true
						queryForm := &carrot.QueryForm{
							Limit: -1,
							Filters: []carrot.Filter{
								{Name: "enabled", Op: "=", Value: true}, // JSON format
							},
							ViewFields: []string{"UUID", "Name"},
						}
						return db, queryForm, nil
					},
				},
			},
		},
	}
}
