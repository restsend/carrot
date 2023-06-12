package main

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"flag"
	"math/rand"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/restsend/carrot"
	"gorm.io/gorm"
)

type ProductItem struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"createdAt"`
	ProductID uint      `json:"-"`
	Product   Product   `json:"product"`
	Name      string    `json:"name" gorm:"size:128"`
	Unit      string    `json:"unit"`
	Price     int       `json:"price"`
}
type ProductModel struct {
	Name  string `json:"name" gorm:"size:40"`
	Image string `json:"image" gorm:"size:200"`
}

func (m *ProductModel) Scan(value interface{}) error {
	return json.Unmarshal(value.([]byte), m)
}

func (m *ProductModel) Value() (driver.Value, error) {
	return json.Marshal(m)
}

type Product struct {
	UUID      string        `json:"id" gorm:"primarykey;size:20"`
	GroupID   int           `json:"-"`
	Name      string        `json:"name" gorm:"size:200"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`
	Enabled   bool          `json:"enabled"`
	Model     *ProductModel `json:"model"`
}

type Customer struct {
	ID        uint       `json:"id" gorm:"primarykey"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	Name      string     `json:"name" gorm:"size:32"`
	Age       int        `json:"age"`
	Enabled   bool       `json:"enabled"`
	OpenedAt  *time.Time `json:"openedAt,omitempty"`
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

	carrot.MakeMigrates(db, []any{
		&Product{},
		&ProductItem{},
		&Customer{},
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
	productAdmins := []carrot.AdminObject{
		{
			Model:       &Product{},
			Group:       "Product",
			Name:        "Product",
			Desc:        "Product is a thing that can be sold or bought ",
			Shows:       []string{"UUID", "Name", "Enabled", "Model", "CreatedAt", "UpdatedAt"},
			Editables:   []string{"UUID", "Name", "Enabled", "Model"},
			Searchables: []string{"UUID", "Name", "Model"},
			Filterables: []string{"Enabled", "CreatedAt", "UpdatedAt"},
		},
		{
			Model:       &ProductItem{},
			Group:       "Product",
			Name:        "ProductItem",
			Desc:        "A item of product",
			Shows:       []string{"ID", "ProductID", "Name", "Unit", "Price", "CreatedAt"},
			Editables:   []string{"ProductID", "Name", "Unit", "Price"},
			Searchables: []string{"Name"},
			Filterables: []string{"Unit", "Price", "CreatedAt"},
		},
		{
			Model:       &Customer{},
			Group:       "Product",
			Name:        "Customer",
			Desc:        "A simple CRM system",
			Shows:       []string{"ID", "Name", "Age", "Enabled", "OpenedAt", "CreatedAt", "UpdatedAt"},
			Editables:   []string{"ID", "Name", "Enabled", "OpenedAt"},
			Searchables: []string{"Name"},
			Filterables: []string{"Enabled", "CreatedAt", "UpdatedAt"},
		},
	}
	adminobjs = append(adminobjs, productAdmins...)
	carrot.RegisterAdmins(r.Group("/admin"), db, carrot.HintAssetsRoot("admin"), adminobjs)
	r.Run(":8080")
}

func GetWebObjects(db *gorm.DB) []carrot.WebObject {
	return []carrot.WebObject{
		{
			Name:        "customer",
			Model:       &Customer{},
			Searchables: []string{"Name", "Enabled"},
			Editables:   []string{"Name", "Age", "Enabled"},
			Filterables: []string{"Name", "CreatedAt", "Age", "Enabled"},
			Orderables:  []string{"CreatedAt", "Age", "Enabled"},
			GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
				return db
			},
		},
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
