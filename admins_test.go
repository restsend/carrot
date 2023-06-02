package carrot

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestAdminSettings(t *testing.T) {
	s := AdminSettings{
		TempalteRoot: "test",
	}
	assert.Equal(t, "test/hello.png", s.hintPage("test", "hello.png"))

	objs := GetCarrotAdminObjects()
	err := objs[0].Build()
	assert.Nil(t, err)
	assert.Equal(t, "ID", objs[0].PrimaryKeyName)
	user := User{
		ID:    1,
		Phone: "+1234567890",
		Email: "bob@restsend.com",
	}
	vals, err := objs[0].MarshalOne(&user)
	assert.Nil(t, err)
	assert.Equal(t, uint(1), vals["ID"])
	assert.Equal(t, "+1234567890", vals["Phone"])
	assert.Equal(t, "bob@restsend.com", vals["Email"])
	assert.Equal(t, false, vals["IsSuperUser"])
	assert.Nil(t, vals["LastLogin"])
	data, err := json.Marshal(vals)
	assert.Nil(t, err)
	assert.Contains(t, string(data), `"LastLogin":null`)
	assert.Contains(t, string(data), `"IsSuperUser":false`)
	assert.Contains(t, string(data), `"CreatedAt":"0001-01-01T00:00:00Z"`)
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
func TestAdminPageObjects(t *testing.T) {
	RegisterCarrotFilters()
	{
		tmpl := `{{objects|stringify}}`
		r, err := pongo2.DefaultSet.RenderTemplateString(tmpl, pongo2.Context{
			"objects": GetCarrotAdminObjects(),
		})
		assert.Nil(t, err)
		assert.Contains(t, r, `"group":"Sys","name":"User"`)
	}
	{
		tmpl := `{{settings|stringify}} {{user|stringify}}`
		r, err := pongo2.DefaultSet.RenderTemplateString(tmpl, pongo2.Context{
			"settings": AdminSettings{
				Title: "Carrot Admin Unittest",
			},
			"user": User{
				IsSuperUser: true,
			},
		})
		assert.Nil(t, err)
		assert.Contains(t, r, `"Carrot Admin Unittest"`)
		assert.Contains(t, r, `"email":""`)
	}
}

func createAdminTest() (*gin.Engine, *gorm.DB, *TestClient) {
	r := gin.Default()
	db, _ := InitDatabase(nil, "", "")
	InitCarrot(db, r)

	as := r.HTMLRender.(*StaticAssets)
	as.Paths = []string{"assets"}
	objs := GetCarrotAdminObjects()
	settings := AdminSettings{
		Title:        "Carrot Admin Unittest",
		TempalteRoot: "admin",
	}
	RegisterAdmins(r.Group("/admin"), as, objs, &settings)
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
	settings := AdminSettings{
		Title:        "Carrot Admin Unittest",
		TempalteRoot: "admin",
	}
	RegisterAdmins(r.Group("/admin"), as, objs, &settings)

	client := NewTestClient(r)

	w := client.Get("/admin/")
	assert.Equal(t, w.Code, 302)
	assert.Equal(t, w.Header().Get("Location"), "/auth/login?next=http://MOCKSERVER/admin/")

	authClient(db, client, "bob@restsend.com", "--", true)

	client.CallPost("/auth/login", &LoginForm{
		Email:    "bob@restsend.com",
		Password: "--",
	}, nil)
	w = client.Get("/admin/")
	assert.Equal(t, w.Code, 200)
	body := w.Body.String()
	assert.Contains(t, body, settings.Title)
	assert.Contains(t, body, `"primary":true`)
	assert.Contains(t, body, `const adminobject = {`)
	assert.Contains(t, body, `objects:[{`)
}

func TestAdminCRUD(t *testing.T) {
	_, db, client := createAdminTest()

	{
		var r Config
		err := client.CallPut("/admin/config/", gin.H{
			"ID":    1024,
			"Key":   "test",
			"Value": "mock",
		}, &r)
		assert.Nil(t, err)
		assert.Equal(t, uint(1024), r.ID)
		assert.Equal(t, "test", r.Key)
		assert.Equal(t, "mock", r.Value)
	}
	{
		var r bool
		err := client.CallPatch("/admin/config/1024", gin.H{
			"ID":    1024,
			"Key":   "test2",
			"Value": "mock2",
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
				Name:  "Key",
				Op:    "like",
				Value: "SITE_SI",
			},
		}
		err = client.CallPost("/admin/config/", &form, &result)
		assert.Nil(t, err)
		//assert.Equal(t, 2, len(result.Items))
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
		assert.Contains(t, first, "Email")
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

func TestAdminRender(t *testing.T) {
	_, _, client := createAdminTest()
	w := client.Post(http.MethodPost, "/admin/config/_/render/list.html", nil)
	assert.Equal(t, w.Code, 200)

	w = client.Post(http.MethodPost, "/admin/config/_/render/builtin.js", nil)
	assert.Equal(t, w.Code, 400)

	w = client.Post(http.MethodPost, "/admin/config/_/render/notexist.html", nil)
	assert.Equal(t, w.Code, 404)
}

func TestAdminSingle(t *testing.T) {
	_, db, client := createAdminTest()
	w := client.Get("/admin/config/")
	assert.Equal(t, w.Code, 200)
	body := w.Body.String()
	assert.Contains(t, body, `"name":"Config"`)

	db.Model(&Config{}).Create(&Config{
		ID:    1024,
		Key:   "Carrot Admin Unittest",
		Value: "mock"})

	w = client.Get("/admin/config/1024")
	assert.Equal(t, w.Code, 200)

	body = w.Body.String()
	assert.Contains(t, body, `"Key":"Carrot`)
	assert.Contains(t, body, `"ID":1024`)
	var config Config
	err := json.Unmarshal([]byte(body), &config)
	assert.Nil(t, err)
	assert.Equal(t, "Carrot Admin Unittest", config.Key)
	assert.Equal(t, "mock", config.Value)
}

func TestAdminAction(t *testing.T) {
}
