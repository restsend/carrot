package carrot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestConvertKey(t *testing.T) {
	v := ConvertKey(reflect.TypeOf(uint64(0)), "1234")
	assert.Equal(t, v, uint64(1234))

	v = ConvertKey(reflect.TypeOf(uint(0)), "1234")
	assert.Equal(t, v, uint(1234))

	v = ConvertKey(reflect.TypeOf(int64(0)), "1234")
	assert.Equal(t, v, int64(1234))

	v = ConvertKey(reflect.TypeOf(int(0)), "1234")
	assert.Equal(t, v, int(1234))

	v = ConvertKey(reflect.TypeOf("1234"), 1234)
	assert.Equal(t, v, "1234")

	v = ConvertKey(reflect.TypeOf("1234"), nil)
	assert.Nil(t, v)

	v = ConvertKey(reflect.TypeOf("1234"), "1234")
	assert.Equal(t, v, "1234")
}

func TestUniqueKey(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []interface{}{&User{}, &Config{}})
	assert.Nil(t, err)
	v := GenUniqueKey(db.Model(User{}), "email", 10)
	assert.Equal(t, len(v), 10)
	v = GenUniqueKey(db.Model(User{}), "xx", 10)
	assert.Equal(t, len(v), 0)
}

func TestFilterOp(t *testing.T) {
	f := Filter{
		Name: "name",
		Op:   FilterOpEqual,
	}
	assert.Equal(t, f.GetQuery(), "name")

	f.Op = FilterOpNotEqual
	assert.Equal(t, f.GetQuery(), "name <> ?")
	f.Op = FilterOpIn
	assert.Equal(t, f.GetQuery(), "name IN ?")
	f.Op = FilterOpNotIn
	assert.Equal(t, f.GetQuery(), "name NOT IN ?")
	{
		f.Value = `["1",2]`
		// f.targetValue = f.Value
		assert.Equal(t, f.GetValue(), []any{"1", float64(2)})
	}

	f.Op = FilterOpGreater
	assert.Equal(t, f.GetQuery(), "name > ?")
	f.Op = FilterOpGreaterOrEqual
	assert.Equal(t, f.GetQuery(), "name >= ?")

	f.Op = FilterOpLess
	assert.Equal(t, f.GetQuery(), "name < ?")
	f.Op = FilterOpLessOrEqual
	assert.Equal(t, f.GetQuery(), "name <= ?")

	o := Order{
		Name: "createdAt",
	}
	assert.Equal(t, o.GetQuery(), "createdAt")
	o.Op = OrderOpDesc
	assert.Equal(t, o.GetQuery(), "createdAt DESC")
}

func TestQueryObjects(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []interface{}{&User{}, &Config{}})
	assert.Nil(t, err)
	bob, _ := CreateUser(db, "bob@example.org", "123456")
	UpdateUserFields(db, bob, map[string]interface{}{"FirstName": "bot"})
	form := QueryForm{
		Pos:   0,
		Limit: 10,
		Filters: []Filter{
			{
				Name:  "email",
				Op:    FilterOpEqual,
				Value: "bob@example.org",
			},
		},
		Orders: []Order{
			{
				Name: "Email",
			},
		},
		searchFields: []string{"first_name"},
		Keyword:      "ot",
	}
	obj := WebObject{
		tableName: "users",
		modelElem: reflect.TypeOf(bob).Elem(),
	}
	r, err := QueryObjects(db.Debug(), &obj, &form)
	assert.Nil(t, err)
	assert.Equal(t, r.TotalCount, 1)
	data, _ := json.Marshal(r.Items)
	assert.Contains(t, string(data), "bob@example.org")
	users, ok := r.Items.([]User)
	assert.True(t, ok)
	assert.Equal(t, len(users), 1)
	assert.Equal(t, r.Limit, 10)
	assert.Equal(t, r.Pos, 1)
}

func TestRegisterWebObject(t *testing.T) {
	type MockUser struct {
		ID   uint   `json:"tid" gorm:"primarykey"`
		Name string `gorm:"size:100"`
	}

	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []interface{}{&MockUser{}})
	assert.Nil(t, err)
	user := MockUser{Name: "user_1"}
	db.Create(&user)

	r := gin.Default()
	RegisterObjects(r, []WebObject{
		{
			Model: MockUser{},
			Name:  "muser",
			GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
				return db
			},
			AllowMethods: WebObjectDelete | WebObjectCreate, // Only register create & delete handler.
		},
	})

	client := NewTestClient(r)

	{
		demo := MockUser{Name: "user_2"}

		// Create
		body, _ := json.Marshal(demo)
		w := client.PostRaw(http.MethodPut, "/muser", body)
		assert.Equal(t, w.Code, http.StatusOK)
		var create MockUser
		err = json.Unmarshal(w.Body.Bytes(), &create)
		assert.Nil(t, err)
		assert.Equal(t, demo.Name, create.Name)

		// Single Query
		w = client.GetRaw(fmt.Sprintf("/muser/%d", create.ID))
		assert.Equal(t, w.Code, http.StatusNotFound)

		// Edit
		w = client.PostRaw(http.MethodPost, fmt.Sprintf("/muser/%d", create.ID), nil)
		assert.Equal(t, w.Code, http.StatusNotFound)

		// Query
		w = client.PostRaw(http.MethodPost, "/muser/query", nil)
		assert.Equal(t, w.Code, http.StatusNotFound)

		// Delete
		w = client.PostRaw(http.MethodDelete, fmt.Sprintf("/muser/%d", create.ID), nil)
		assert.Equal(t, w.Code, http.StatusOK)
	}
}

func TestWebObject(t *testing.T) {
	type MockUser struct {
		ID          uint   `json:"tid" gorm:"primarykey"`
		Name        string `gorm:"size:100"`
		Age         int
		DisplayName string `json:"nick" gorm:"size:100"`
		Body        string `json:"-" gorm:"-"`
	}

	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []interface{}{&MockUser{}})
	assert.Nil(t, err)
	user := MockUser{Name: "user_1", Age: 10}
	db.Create(&user)

	r := gin.Default()
	RegisterObjects(r, []WebObject{
		{
			Model:     MockUser{},
			Name:      "muser",
			Editables: []string{"Name", "DisplayName"},
			Filters:   []string{"Name", "Age"},
			Orders:    []string{"Name"},
			Searchs:   []string{"Name"},
			GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
				return db
			},
			Init: func(ctx *gin.Context, vptr interface{}) {},
		},
	})
	client := NewTestClient(r)
	{
		// Create
		user2 := MockUser{Name: "user_2", Age: 11}
		body, _ := json.Marshal(&user2)
		req, _ := http.NewRequest(http.MethodPut, "/muser", bytes.NewReader(body))
		w := client.SendReq("/muser", req)
		assert.Equal(t, w.Code, http.StatusOK)

		var u2 MockUser
		err = json.Unmarshal(w.Body.Bytes(), &u2)
		assert.Nil(t, err)
		assert.Equal(t, u2.Name, "user_2")

		// Get after create
		w = client.GetRaw(fmt.Sprintf("/muser/%d", u2.ID))
		assert.Equal(t, w.Code, http.StatusOK)

		err = json.Unmarshal(w.Body.Bytes(), &u2)
		assert.Nil(t, err)
		assert.Equal(t, u2.Name, "user_2")

		// Edit
		body, _ = json.Marshal(map[string]string{"DisplayName": "test"})

		req, _ = http.NewRequest(http.MethodPatch, fmt.Sprintf("/muser/%d", u2.ID), bytes.NewReader(body))
		w = client.SendReq(fmt.Sprintf("/muser/%d", u2.ID), req)
		assert.Equal(t, w.Code, http.StatusBadRequest)
		assert.Contains(t, w.Body.String(), "not changed")

		body, _ = json.Marshal(map[string]string{"nick": "test"})
		req, _ = http.NewRequest(http.MethodPatch, fmt.Sprintf("/muser/%d", u2.ID), bytes.NewReader(body))
		w = client.SendReq(fmt.Sprintf("/muser/%d", u2.ID), req)
		assert.Equal(t, w.Code, http.StatusOK)

		// query
		form := QueryForm{
			Filters: []Filter{
				{
					Name:  "Age",
					Op:    FilterOpGreaterOrEqual,
					Value: "11",
				},
			},
			Orders: []Order{
				{Name: "Name", Op: OrderOpDesc},
			},
			Keyword: "_2",
		}
		var result QueryResult
		err = client.Post("/muser/query", &form, &result)
		assert.Nil(t, err)
		assert.Equal(t, result.TotalCount, 1)
		assert.Equal(t, result.Pos, 1)

		// Delete
		req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("/muser/%d", u2.ID), nil)
		w = client.SendReq(fmt.Sprintf("/muser/%d", u2.ID), req)
		assert.Equal(t, w.Code, http.StatusOK)

		// Will not found
		w = client.GetRaw(fmt.Sprintf("/muser/%d", u2.ID))
		assert.Equal(t, w.Code, http.StatusNotFound)
	}
}

func TestRpcCall(t *testing.T) {
	type MockUser struct {
		ID   uint   `json:"tid" gorm:"primarykey"`
		Name string `gorm:"size:100"`
		Age  int
	}

	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []interface{}{&MockUser{}})
	assert.Nil(t, err)
	user := MockUser{Name: "user_1", Age: 10}
	db.Create(&user)

	r := gin.Default()
	RegisterObjects(r, []WebObject{
		{
			Model:     MockUser{},
			Name:      "muser",
			Editables: []string{"Name"},
			Filters:   []string{"Name", "Age"},
			Searchs:   []string{"Name"},
			GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
				return db
			},
			Init: func(ctx *gin.Context, vptr interface{}) {},
		},
	})

	client := NewTestClient(r)
	{
		demo := MockUser{Name: "user_2", Age: 11}

		// Create
		var create MockUser
		err := client.Put("/muser", demo, &create)
		assert.Nil(t, err)
		assert.Equal(t, demo.Name, create.Name)

		// Single Query
		var single MockUser
		client.Get(fmt.Sprintf("/muser/%d", create.ID), &single)
		assert.Equal(t, demo.Name, single.Name)

		// Edit
		single.Name = "edited_user"
		err = client.Patch(fmt.Sprintf("/muser/%d", single.ID), single)
		assert.Nil(t, err)

		// Query
		form := QueryForm{
			Filters: []Filter{
				{Name: "Age", Op: FilterOpEqual, Value: "11"},
			},
			Keyword: "edited",
		}
		var query QueryResult
		err = client.Post("/muser/query", &form, &query)
		assert.Nil(t, err)
		assert.Equal(t, 1, query.TotalCount)
		assert.Equal(t, 1, query.Pos)

		// Delete
		err = client.Delete(fmt.Sprintf("/muser/%d", create.ID))
		assert.Nil(t, err)
	}
}

func TestEditBool(t *testing.T) {
	type MockUser struct {
		ID      uint   `json:"tid" gorm:"primarykey"`
		Name    string `json:"name" gorm:"size:99"`
		Enabled bool   `json:"enabled"`
	}

	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []interface{}{&MockUser{}})
	assert.Nil(t, err)
	user := MockUser{Name: "user_1", Enabled: false}
	db.Create(&user)

	r := gin.Default()
	RegisterObjects(r, []WebObject{
		{
			Model:     MockUser{},
			Name:      "muser",
			Editables: []string{"Name", "Enabled"},
			Filters:   []string{"Name", "Enabled"},
			Searchs:   []string{"Name"},
			GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
				return db
			},
			Init: func(ctx *gin.Context, vptr interface{}) {},
		},
	})

	client := NewTestClient(r)
	{
		// Mock data
		var create MockUser = MockUser{Enabled: true}
		err := client.Put("/muser", MockUser{Name: "muser"}, &create)
		assert.Nil(t, err)
		assert.Equal(t, "muser", create.Name)

		tests := []struct {
			name   string
			param  any
			expect bool
		}{
			{"base case1", map[string]any{"enabled": "true"}, true},
			{"base case2", map[string]any{"enabled": "1"}, true},
			{"base case3", map[string]any{"enabled": "t"}, true},
			{"base case4", map[string]any{"enabled": "f"}, false},
			{"base case5", map[string]any{"enabled": "FALSE"}, false},
			{"base case6", map[string]any{"enabled": true}, true},
			{"base case7", map[string]any{"enabled": false}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body, _ := json.Marshal(tt.param)
				w := client.PostRaw(http.MethodPatch, fmt.Sprintf("/muser/%d", create.ID), body)
				assert.Equal(t, w.Code, http.StatusOK)
				if w.Code != http.StatusOK {
					t.Log(w.Body)
				}

				var res MockUser
				client.Get(fmt.Sprintf("/muser/%d", create.ID), &res)
				assert.Equal(t, tt.expect, res.Enabled)
			})
		}

		tests = []struct {
			name   string
			param  any
			expect bool
		}{
			{"bad case 1", map[string]any{"other": true}, false},
			{"bad case 2", map[string]any{"enabled": ""}, false},
			{"bad case 3", map[string]any{"enabled": "xxx"}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body, _ := json.Marshal(tt.param)
				w := client.PostRaw(http.MethodPatch, fmt.Sprintf("/muser/%d", create.ID), body)
				assert.NotEqual(t, w.Code, http.StatusOK)
				fmt.Println(w.Body) // {"error":"not changed"}
			})
		}

	}
}
