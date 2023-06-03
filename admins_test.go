package carrot

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestAdminObjects(t *testing.T) {
	db, _ := InitDatabase(nil, "", "")
	objs := GetCarrotAdminObjects()
	err := objs[0].Build(db)
	assert.Nil(t, err)
	assert.Equal(t, "id", objs[0].PrimaryKey)
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

	as := r.HTMLRender.(*StaticAssets)
	as.Paths = []string{"assets"}
	objs := GetCarrotAdminObjects()
	RegisterAdmins(r.Group("/admin"), db, as, objs)
	client := NewTestClient(r)
	authClient(db, client, "bob@restsend.com", "--", true)
	return r, db, client
}

func TestAdminIndex(t *testing.T) {
	r := gin.Default()
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	InitCarrot(db, r)

	as := r.HTMLRender.(*StaticAssets)
	as.Paths = []string{"assets"}
	objs := GetCarrotAdminObjects()
	RegisterAdmins(r.Group("/admin"), db, as, objs)

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
		var r Config
		err := client.CallPut("/admin/config/", gin.H{
			"id":    1024,
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
		err := client.CallPatch("/admin/config/1024", gin.H{
			"id":    1024,
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
		var form QueryForm
		var result AdminQueryResult
		err := client.CallPost("/admin/user/", &form, &result)
		assert.Nil(t, err)
		var totalcount int64
		err = db.Model(&User{}).Count(&totalcount).Error
		assert.Nil(t, err)
		assert.Equal(t, int(totalcount), result.TotalCount)
		assert.Equal(t, 1, len(result.Items))
		first := result.Items[0]
		assert.Contains(t, first, "email")
	}
	{
		var totalcount int64
		db.Model(&Config{}).Count(&totalcount)

		var r bool
		err := client.CallDelete("/admin/config/1024", nil, &r)
		assert.Nil(t, err)
		assert.True(t, r)

		var totalcount2 int64
		db.Model(&Config{}).Count(&totalcount2)
		assert.Equal(t, totalcount-1, totalcount2)
	}
}

func TestAdminSingle(t *testing.T) {
	_, db, client := createAdminTest()

	db.Model(&Config{}).Create(&Config{
		ID:    1024,
		Key:   "Carrot Admin Unittest",
		Value: "mock"})

	w := client.Post(http.MethodPost, "/admin/config/1024", nil)
	assert.Equal(t, w.Code, 200)

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
}
