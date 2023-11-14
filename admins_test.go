package carrot

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

type ProductModel struct {
	Name  string `json:"name" gorm:"size:40"`
	Image string `json:"image" gorm:"size:200"`
}

func (s ProductModel) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *ProductModel) Scan(input interface{}) error {
	return json.Unmarshal(input.([]byte), s)
}

type ProductItem struct {
	ID         uint          `json:"id" gorm:"primaryKey"`
	Name       string        `json:"name" gorm:"size:40"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  *time.Time    `json:"updated_at"`
	ModelPtr   *ProductModel `json:"model_ptr"`
	ModelValue ProductModel  `json:"model_value"`
}

type Product struct {
	UUID          string      `json:"id" gorm:"primarykey;size:20"`
	ItemID        uint        `json:"-"`
	Item          ProductItem `json:"product_item"`
	invalid_field AdminActionHandler
	Func_field    AdminActionHandler
}

func (item ProductItem) String() string {
	return fmt.Sprintf("%d (%s)", item.ID, item.Name)
}

func TestAdminObjects(t *testing.T) {
	db, _ := InitDatabase(nil, "", "")
	objs := GetCarrotAdminObjects()
	err := objs[0].Build(db)
	assert.Nil(t, err)
	assert.Equal(t, []string{"id", "email"}, objs[0].PrimaryKey)
	user := User{
		ID:    1,
		Phone: "+1234567890",
		Email: "bob@restsend.com",
	}
	vals, err := objs[0].MarshalOne(&user)
	assert.Nil(t, err)
	assert.Equal(t, uint(1), vals["id"])
	assert.Equal(t, "+1234567890", vals["phone"])
	assert.Equal(t, "bob@restsend.com", vals["email"])
	assert.Equal(t, false, vals["is_super_user"])
	assert.Nil(t, vals["last_login"])
	data, err := json.Marshal(vals)
	assert.Nil(t, err)
	assert.Contains(t, string(data), `"last_login":null`)
	assert.Contains(t, string(data), `"is_super_user":false`)
	assert.Contains(t, string(data), `"created_at":"0001-01-01T00:00:00Z"`)
}

func authClient(db *gorm.DB, client *TestClient, email, password string, isSuper bool) {
	if _, err := GetUserByEmail(db, email); err != nil {
		u, _ := CreateUser(db, email, password)
		if isSuper {
			u.IsStaff = true
			u.IsSuperUser = true
			db.Save(&u)
		}
	}

	client.CallPost("/auth/login", &LoginForm{
		Email:    email,
		Password: password,
	}, nil)
}

func createAdminTest() (*gin.Engine, *gorm.DB, *TestClient) {
	r := gin.Default()
	db, _ := InitDatabase(nil, "", "")
	InitCarrot(db, r)

	objs := GetCarrotAdminObjects()
	RegisterAdmins(r.Group("/admin"), db, "./admin", objs)
	client := NewTestClient(r)
	authClient(db, client, "bob@restsend.com", "--", true)
	return r, db, client
}

func TestAdminIndex(t *testing.T) {
	r := gin.Default()
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	InitCarrot(db, r)

	objs := GetCarrotAdminObjects()
	RegisterAdmins(r.Group("/admin"), db, HintAssetsRoot("admin"), objs)

	client := NewTestClient(r)

	w := client.Get("/admin/admin.json")
	assert.Equal(t, w.Code, 302)
	assert.Equal(t, w.Header().Get("Location"), "/auth/login?next=http://MOCKSERVER/admin/admin.json")

	authClient(db, client, "bob@restsend.com", "--", true)

	client.CallPost("/auth/login", &LoginForm{
		Email:    "bob@restsend.com",
		Password: "--",
	}, nil)

	w = client.Post(http.MethodPost, "/admin/admin.json", nil)
	assert.Equal(t, w.Code, 200)
	body := w.Body.String()
	assert.Contains(t, body, "/admin/user")
	assert.Contains(t, body, `"pluralName":"Configs"`)

	w = client.Get("/admin/")
	assert.Equal(t, w.Code, 200)
	body = w.Body.String()
	assert.Contains(t, body, "Admin panel")
}

func TestAdminCRUD(t *testing.T) {
	_, db, client := createAdminTest()

	{
		db.Model(&User{}).Where("email", "bob@restsend.com").UpdateColumn("is_staff", false)
		db.Model(&User{}).Where("email", "bob@restsend.com").UpdateColumn("is_super_user", false)
		err := client.CallPut("/admin/config/?id=1024", gin.H{
			"id": 1024,
		}, nil)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "forbidden")

		db.Model(&User{}).Where("email", "bob@restsend.com").UpdateColumn("is_staff", true)
		db.Model(&User{}).Where("email", "bob@restsend.com").UpdateColumn("is_super_user", false)
		err = client.CallPut("/admin/config/", gin.H{
			"id": 1024,
		}, nil)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "only superuser can access")
	}

	{
		db.Model(&User{}).Where("email", "bob@restsend.com").UpdateColumn("is_super_user", true)
		var r Config
		err := client.CallPut("/admin/config/?id=1024", gin.H{
			"key":   "test",
			"value": "mock",
		}, &r)
		assert.Nil(t, err)
		assert.Equal(t, uint(1024), r.ID)
		assert.Equal(t, "test", r.Key)
		assert.Equal(t, "mock", r.Value)
	}
	{
		var r bool
		err := client.CallPatch("/admin/config/?id=1024", gin.H{
			"key":   "test2",
			"value": "mock2",
		}, &r)
		assert.Nil(t, err)
		assert.True(t, r)
	}
	{
		var result AdminQueryResult
		var form QueryForm
		form.Keyword = "test2"
		err := client.CallPost("/admin/config/", &form, &result)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(result.Items))

		form.Keyword = ""
		form.Filters = []Filter{
			{
				Name:  "key",
				Op:    "like",
				Value: "test",
			},
			{
				Name:  "id",
				Op:    ">=",
				Value: "1024",
			},
		}
		form.Orders = []Order{
			{
				Name: "id",
				Op:   "desc",
			},
		}
		err = client.CallPost("/admin/config/", &form, &result)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(result.Items))
	}
	{
		var result AdminQueryResult
		var form QueryForm
		form.Keyword = ""
		form.Filters = []Filter{
			{
				Name:  "key",
				Op:    "like",
				Value: []string{"test", "test2"},
			},
			{
				Name:  "id",
				Op:    ">=",
				Value: "1024",
			},
		}
		form.Orders = []Order{
			{
				Name: "id",
				Op:   "desc",
			},
		}
		err := client.CallPost("/admin/config/", &form, &result)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(result.Items))
	}
	{
		var form QueryForm
		var result AdminQueryResult
		err := client.CallPost("/admin/user/", &form, &result)
		assert.Nil(t, err)
		var totalCount int64
		err = db.Model(&User{}).Count(&totalCount).Error
		assert.Nil(t, err)
		assert.Equal(t, int(totalCount), result.TotalCount)
		assert.Equal(t, 1, len(result.Items))
		first := result.Items[0]
		assert.Contains(t, first, "email")
	}
	{
		var totalCount int64
		db.Model(&Config{}).Count(&totalCount)

		var r bool
		err := client.CallDelete("/admin/config/?id=1024", nil, &r)
		assert.Nil(t, err)
		assert.True(t, r)

		var totalCount2 int64
		db.Model(&Config{}).Count(&totalCount2)
		assert.Equal(t, totalCount-1, totalCount2)
	}
}

func TestAdminSingle(t *testing.T) {
	_, db, client := createAdminTest()

	db.Model(&Config{}).Create(&Config{
		ID:    1024,
		Key:   "Carrot Admin Unittest",
		Value: "mock"})

	w := client.Post(http.MethodPost, "/admin/config/?id=1024", nil)
	assert.Equal(t, 200, w.Code)

	body := w.Body.String()
	assert.Contains(t, body, `"key":"Carrot`)
	assert.Contains(t, body, `"id":1024`)
	var config Config
	err := json.Unmarshal([]byte(body), &config)
	assert.Nil(t, err)
	assert.Equal(t, "Carrot Admin Unittest", config.Key)
	assert.Equal(t, "mock", config.Value)
}

func TestAdminAction(t *testing.T) {
	_, db, client := createAdminTest()
	CreateUser(db, "alice@restsend.com", "1")
	{
		var r bool
		err := client.CallPost("/admin/user/toggle_enabled?email=alice@restsend.com", nil, &r)
		assert.Nil(t, err)
		assert.False(t, r)
		u, _ := GetUserByEmail(db, "alice@restsend.com")
		assert.False(t, u.Enabled)
	}
	{
		var r bool
		err := client.CallPost("/admin/user/toggle_staff?email=alice@restsend.com", nil, &r)
		assert.Nil(t, err)
		assert.True(t, r)
		u, _ := GetUserByEmail(db, "alice@restsend.com")
		assert.True(t, u.IsStaff)
	}
	{
		var r bool
		err := client.CallPost("/admin/user/bad_action?email=alice@restsend.com", nil, &r)
		assert.Contains(t, err.Error(), "400 Bad Request")
		assert.False(t, r)
	}
}

func TestAdminFieldMarshal(t *testing.T) {

	obj := AdminObject{
		Model: &ProductItem{},
		Path:  "unittest",
	}
	db, _ := InitDatabase(nil, "", "")
	MakeMigrates(db, []any{&ProductItem{}})
	err := obj.Build(db)
	assert.Nil(t, err)
	elemObj := reflect.New(obj.modelElem)
	v, err := obj.UnmarshalFrom(elemObj, nil, map[string]any{
		"id":         1024,
		"name":       "mock item",
		"created_at": "2020-01-01T00:00:00Z",
		"updated_at": "2020-01-01T00:00:00Z",
		"model_ptr": map[string]any{
			"name": "test", "image": "http://test.com",
		},
		"model_value": map[string]any{
			"name": "test2", "image": "http://test.com",
		},
	})
	assert.Nil(t, err)
	assert.Equal(t, "test", v.(*ProductItem).ModelPtr.Name)
	assert.Equal(t, "test2", v.(*ProductItem).ModelValue.Name)
}

func TestAdminForeign(t *testing.T) {
	productObj := AdminObject{
		Model: &Product{},
		Path:  "unittest",
	}
	db, _ := InitDatabase(nil, "", "")
	MakeMigrates(db, []any{&ProductItem{}, &Product{}})

	err := productObj.Build(db)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(productObj.Fields))
	assert.Equal(t, "item", productObj.Fields[1].Name)
	assert.Equal(t, "item_id", productObj.Fields[1].Foreign.Field)
	assert.Equal(t, "productitem", productObj.Fields[1].Foreign.Path)

	p := Product{
		UUID:   "test",
		ItemID: 1024,
		Item: ProductItem{
			ID:   1024,
			Name: "item one",
		},
	}
	assert.Equal(t, "1024 (item one)", fmt.Sprintf("%v", p.Item))
	vals, err := productObj.MarshalOne(&p)
	assert.Nil(t, err)
	assert.Equal(t, "1024 (item one)", vals["item"].(AdminValue).Label)
	assert.Equal(t, uint(1024), vals["item"].(AdminValue).Value)
}

func TestAdminConvert(t *testing.T) {
	{
		var x int64
		v, err := convertValue(reflect.TypeOf(x), 1.000000001)
		assert.Nil(t, err)
		assert.Equal(t, int64(1), v)
	}
	{
		var x int64
		v, err := convertValue(reflect.TypeOf(x), 2)
		assert.Nil(t, err)
		assert.Equal(t, int64(2), v)
	}
}

func TestParseField(t *testing.T) {
	productObj := AdminObject{
		Model: &Product{},
		Path:  "unittest",
	}
	db, _ := InitDatabase(nil, "", "")
	MakeMigrates(db, []any{&ProductItem{}, &Product{}})

	err := productObj.Build(db)
	assert.Nil(t, err)
	assert.Equal(t, len(productObj.Fields), 2)
}

func TestAdminUpdatePrimaryKeys(t *testing.T) {
	type UniqueItem struct {
		ID         string        `json:"id" gorm:"uniqueIndex:idx_id_name"`
		Name       string        `json:"name" gorm:"size:40;uniqueIndex:idx_id_name"`
		JoinedAt   time.Time     `json:"joined_at"`
		UpdatedAt  *time.Time    `json:"updated_at"`
		ModelPtr   *ProductModel `json:"model_ptr"`
		ModelValue ProductModel  `json:"model_value"`
	}

	itemObj := AdminObject{
		Model: &UniqueItem{},
		Path:  "unittest",
	}
	db, _ := InitDatabase(nil, "", "")
	db = db.Debug()
	MakeMigrates(db.Debug(), []any{&UniqueItem{}})
	err := itemObj.Build(db)
	assert.Nil(t, err)
	itemObj.GetDB = func(c *gin.Context, isCreate bool) *gorm.DB {
		return db
	}
	r := gin.Default()
	{
		w := httptest.NewRecorder()
		c := gin.CreateTestContextOnly(w, r)
		body := []byte(`{ "name": "test", "joined_at": "2018-09-10T11:02:00Z" }`)
		c.Request, _ = http.NewRequest(http.MethodPut, "/unittest/?id=100", bytes.NewBuffer(body))
		c.Request.Header.Add("Content-Type", "application/json")
		itemObj.handleCreate(c)
		assert.Equal(t, http.StatusOK, c.Writer.Status())
	}

	{
		w := httptest.NewRecorder()
		c := gin.CreateTestContextOnly(w, r)
		body := []byte(`{"joined_at": "2000-09-10T11:02:00Z"}`)
		c.Request, _ = http.NewRequest(http.MethodPatch, "/unittest/?id=100&name=test", bytes.NewBuffer(body))
		c.Request.Header.Add("Content-Type", "application/json")
		itemObj.handleUpdate(c)

		var total int64
		itemObj.GetDB(c, false).Model(&UniqueItem{}).Count(&total)

		assert.Equal(t, 1, int(total))

		assert.Equal(t, http.StatusOK, c.Writer.Status())
		var obj UniqueItem
		r := itemObj.GetDB(c, false).Where("id", 100).Where("name", "test").Take(&obj)
		assert.Nil(t, r.Error)
		assert.Equal(t, 2000, obj.JoinedAt.Year())
	}

	{
		w := httptest.NewRecorder()
		c := gin.CreateTestContextOnly(w, r)
		body := []byte(`{ "name": "test101",  "id":101 }`)
		c.Request, _ = http.NewRequest(http.MethodPatch, "/unittest/?id=100&name=test", bytes.NewBuffer(body))
		c.Request.Header.Add("Content-Type", "application/json")
		itemObj.handleUpdate(c)
		assert.Equal(t, http.StatusOK, c.Writer.Status())
		var obj UniqueItem
		r := itemObj.GetDB(c, false).Where("id", 101).Where("name", "test101").Take(&obj)
		assert.Nil(t, r.Error)
		assert.Equal(t, "test101", obj.Name)
	}
}
