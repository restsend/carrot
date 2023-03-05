package carrot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// func TestConvertKey(t *testing.T) {
// 	v := ConvertKey(reflect.TypeOf(uint64(0)), "1234")
// 	assert.Equal(t, v, uint64(1234))

// 	v = ConvertKey(reflect.TypeOf(uint(0)), "1234")
// 	assert.Equal(t, v, uint(1234))

// 	v = ConvertKey(reflect.TypeOf(int64(0)), "1234")
// 	assert.Equal(t, v, int64(1234))

// 	v = ConvertKey(reflect.TypeOf(int(0)), "1234")
// 	assert.Equal(t, v, int(1234))

// 	v = ConvertKey(reflect.TypeOf("1234"), 1234)
// 	assert.Equal(t, v, "1234")

// 	v = ConvertKey(reflect.TypeOf("1234"), nil)
// 	assert.Nil(t, v)

// 	v = ConvertKey(reflect.TypeOf("1234"), "1234")
// 	assert.Equal(t, v, "1234")
// }

// func TestFilterOp(t *testing.T) {
// 	f := Filter{
// 		Name: "name",
// 		Op:   FilterOpEqual,
// 	}
// 	assert.Equal(t, f.GetQuery(), "name")

// 	f.Op = FilterOpNotEqual
// 	assert.Equal(t, f.GetQuery(), "name <> ?")
// 	f.Op = FilterOpIn
// 	assert.Equal(t, f.GetQuery(), "name IN ?")
// 	f.Op = FilterOpNotIn
// 	assert.Equal(t, f.GetQuery(), "name NOT IN ?")
// 	.Op = FilterOpGreater
// 	assert.Equal(t, f.GetQuery(), "name > ?")
// 	f.Op = FilterOpGreaterOrEqual
// 	assert.Equal(t, f.GetQuery(), "name >= ?")

// 	f.Op = FilterOpLess
// 	assert.Equal(t, f.GetQuery(), "name < ?")
// 	f.Op = FilterOpLessOrEqual
// 	assert.Equal(t, f.GetQuery(), "name <= ?")

// 	o := Order{
// 		Name: "createdAt",
// 	}
// 	assert.Equal(t, o.GetQuery(), "createdAt")
// 	o.Op = OrderOpDesc
// 	assert.Equal(t, o.GetQuery(), "createdAt DESC")
// }

func TestQueryObjects(t *testing.T) {
	db, err := InitDatabase(nil, "", "")
	MakeMigrates(db, []any{&User{}, &Config{}})
	assert.Nil(t, err)
	bob, _ := CreateUser(db, "bob@example.org", "123456")
	UpdateUserFields(db, bob, map[string]any{"FirstName": "bot"})
	form := QueryForm{
		Pos:          0,
		Limit:        10,
		Filters:      []Filter{{Name: "email", Op: "=", Value: "bob@example.org"}},
		Orders:       []Order{{Name: "Email"}},
		searchFields: []string{"first_name"},
		Keyword:      "ot",
	}
	obj := WebObject[User]{
		tableName: "users",
	}
	r, err := QueryObjects(db, &obj, &form)
	assert.Nil(t, err)
	assert.Equal(t, r.TotalCount, 1)
	data, _ := json.Marshal(r.Items)
	assert.Contains(t, string(data), "bob@example.org")

	users := r.Items
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
	MakeMigrates(db, []any{&MockUser{}})
	assert.Nil(t, err)
	user := MockUser{Name: "user_1"}
	db.Create(&user)

	r := gin.Default()
	obj := WebObject[MockUser]{
		Name:         "muser",
		GetDB:        func(ctx *gin.Context, isCreate bool) *gorm.DB { return db },
		AllowMethods: DELETE | CREATE, // Only register create & delete handler.
	}
	obj.RegisterObject(r)

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
	MakeMigrates(db, []any{&MockUser{}})
	assert.Nil(t, err)
	user := MockUser{Name: "user_1", Age: 10}
	db.Create(&user)

	r := gin.Default()
	obj := WebObject[MockUser]{
		Name:      "muser",
		Editables: []string{"Name", "DisplayName"},
		Filters:   []string{"Name", "Age"},
		Orders:    []string{"Name"},
		Searchs:   []string{"Name"},
		GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return db
		},
		Init: func(ctx *gin.Context, v *MockUser) {},
	}
	obj.RegisterObject(r)

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
		var result QueryResult[MockUser]
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
	MakeMigrates(db, []any{&MockUser{}})
	assert.Nil(t, err)
	user := MockUser{Name: "user_1", Age: 10}
	db.Create(&user)

	r := gin.Default()
	obj := WebObject[MockUser]{
		Name:      "muser",
		Editables: []string{"Name"},
		Filters:   []string{"Name", "Age"},
		Searchs:   []string{"Name"},
		GetDB:     func(ctx *gin.Context, isCreate bool) *gorm.DB { return db },
		Init:      func(ctx *gin.Context, v *MockUser) {},
	}
	obj.RegisterObject(r)

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
		var query QueryResult[MockUser]
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
	MakeMigrates(db, []any{&MockUser{}})
	assert.Nil(t, err)
	user := MockUser{Name: "user_1", Enabled: false}
	db.Create(&user)

	r := gin.Default()
	obj := WebObject[MockUser]{
		Name:      "muser",
		Editables: []string{"Name", "Enabled"},
		Filters:   []string{"Name", "Enabled"},
		Searchs:   []string{"Name"},
		GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return db
		},
		Init: func(ctx *gin.Context, v *MockUser) {},
	}
	obj.RegisterObject(r)

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
			{"base case1", map[string]any{"enabled": true}, true},
			{"base case2", map[string]any{"enabled": false}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body, _ := json.Marshal(tt.param)
				w := client.PostRaw(http.MethodPatch, fmt.Sprintf("/muser/%d", create.ID), body)
				assert.Equal(t, w.Code, http.StatusOK)

				var res MockUser
				client.Get(fmt.Sprintf("/muser/%d", create.ID), &res)
				assert.Equal(t, tt.expect, res.Enabled)
			})
		}

		badtests := []struct {
			name  string
			param any
		}{
			{"bad case 1", map[string]any{"other": true}},
			{"bad case 2", map[string]any{"enabled": ""}},
			{"bad case 3", map[string]any{"enabled": "xxx"}},
			{"bad case3", map[string]any{"enabled": "t"}},
			{"bad case4", map[string]any{"enabled": "f"}},
			{"bad case5", map[string]any{"enabled": 1}},
		}

		for _, tt := range badtests {
			t.Run(tt.name, func(t *testing.T) {
				body, _ := json.Marshal(tt.param)
				w := client.PostRaw(http.MethodPatch, fmt.Sprintf("/muser/%d", create.ID), body)
				assert.Equal(t, w.Code, http.StatusBadRequest)
			})
		}

	}
}

func TestObjectCRUD(t *testing.T) {
	type User struct {
		ID   uint   `json:"uid" gorm:"primarykey"`
		Name string `gorm:"size:100"`
		Age  int
		Body string `json:"-" gorm:"-"`
	}

	db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
	db.AutoMigrate(User{})
	err := db.Create(&User{ID: 1, Name: "user", Age: 10}).Error
	assert.Nil(t, err)

	r := gin.Default()
	webobject := WebObject[User]{
		Editables: []string{"Name"},
		Filters:   []string{"Name"},
		Searchs:   []string{"Name"},
		GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return db.Debug()
		},
		Init: func(ctx *gin.Context, vptr *User) {},
	}
	err = webobject.RegisterObject(r)
	assert.Nil(t, err)

	// Create
	{
		b, _ := json.Marshal(User{Name: "add"})
		req := httptest.NewRequest(http.MethodPut, "/user", bytes.NewReader(b))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Contains(t, w.Body.String(), `"uid":2`)
	}
	// Single Query
	{
		req := httptest.NewRequest(http.MethodGet, "/user/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Contains(t, w.Body.String(), `"uid":1`)
	}
	// Update
	{
		b, _ := json.Marshal(User{Name: "update", Age: 11})
		req := httptest.NewRequest(http.MethodPatch, "/user/1", bytes.NewReader(b))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		assert.Equal(t, "true", w.Body.String())
	}
	// Query
	{
		data := map[string]any{
			"pos":     0,
			"limit":   5,
			"keyword": "",
			"filters": []map[string]any{
				{
					"name":  "Name",
					"op":    "=",
					"value": "update",
				},
			},
		}
		b, _ := json.Marshal(data)
		req := httptest.NewRequest(http.MethodPost, "/user/query", bytes.NewReader(b))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		var res QueryResult[User]
		err := json.Unmarshal(w.Body.Bytes(), &res)
		assert.Nil(t, err)
		assert.Equal(t, 1, res.TotalCount)
		assert.Equal(t, "update", res.Items[0].Name)
	}
	// Delete
	{
		req := httptest.NewRequest(http.MethodDelete, "/user/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	}
	// Query After Delete
	{
		req := httptest.NewRequest(http.MethodGet, "/user/1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)

		b, _ := json.Marshal(map[string]any{"pos": 0, "limit": 5})
		req = httptest.NewRequest(http.MethodPost, "/user/query", bytes.NewReader(b))
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)

		var res QueryResult[User]
		err := json.Unmarshal(w.Body.Bytes(), &res)
		assert.Nil(t, err)
		assert.Equal(t, 1, res.TotalCount)
	}
}

func TestObjectQuery(t *testing.T) {
	type Super struct {
		Fly bool
	}
	type User struct {
		ID       uint      `json:"uid" gorm:"primarykey"`
		Name     string    `json:"name" gorm:"size:100"`
		Body     string    `json:"-" gorm:"-"`
		Birthday time.Time `json:"birthday"`
		Enabled  bool      `json:"enabled"`
		Age      int
		Super
	}

	db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
	db.AutoMigrate(User{})

	r := gin.Default()
	webobject := WebObject[User]{
		Filters: []string{"Name", "Age", "Birthday", "Enabled"},
		Searchs: []string{"Name"},
		GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return db.Debug()
		},
		Init: func(ctx *gin.Context, vptr *User) {},
	}
	err := webobject.RegisterObject(r)
	assert.Nil(t, err)

	// Mock
	{
		db.Create(&User{ID: 1, Name: "alice", Age: 10, Enabled: true, Birthday: time.Now()})
		db.Create(&User{ID: 2, Name: "bob", Age: 10, Enabled: true, Birthday: time.Now()})
		db.Create(&User{ID: 3, Name: "foo", Age: 13})
		db.Create(&User{ID: 4, Name: "bar", Age: 13})
	}
	// Query
	{
		type Param struct {
			Keyword string
			Filters []map[string]any
		}
		type Except struct {
			Num int
		}
		tests := []struct {
			name   string
			params Param
			expect Except
		}{
			{"base_case_1",
				Param{Keyword: "", Filters: nil},
				Except{4},
			},
			{"base_case_2",
				Param{Keyword: "bob", Filters: nil},
				Except{1},
			},
			{"base_case_3",
				Param{Keyword: "", Filters: []map[string]any{
					{"name": "name", "op": "=", "value": "alice"},
				}},
				Except{1},
			},
			{
				"base_case_4",
				Param{Keyword: "", Filters: []map[string]any{
					{"name": "Age", "op": ">=", "value": "10"}},
				},
				Except{4},
			},
			{
				"base_case_5: multiple filters",
				Param{Keyword: "", Filters: []map[string]any{
					{"name": "Age", "op": ">", "value": "11"},
					{"name": "Age", "op": "<", "value": "15"}},
				},
				Except{2},
			},
			{
				"base_case_6:",
				Param{Keyword: "", Filters: []map[string]any{
					{"name": "Age", "op": ">", "value": "11"},
					{"name": "Age", "op": "<", "value": "15"}},
				},
				Except{2},
			},
			{
				"base_case_7:",
				Param{Keyword: "", Filters: []map[string]any{
					{"name": "name", "op": "in", "value": []any{"alice", "bob", "foo"}}},
				},
				Except{3},
			},
			{
				"base_case_8:",
				Param{Keyword: "", Filters: []map[string]any{
					{"name": "name", "op": "in", "value": []any{"alice", "bob"}},
					{"name": "Age", "op": "<>", "value": "10"}},
				},
				Except{0},
			},
			{
				"base_case_9:",
				Param{Keyword: "", Filters: []map[string]any{
					{"name": "birthday", "op": ">=", "value": "2023-01-01"}},
				},
				Except{2},
			},
			{
				"bool_case_1",
				Param{Filters: []map[string]any{
					{"name": "enabled", "op": "=", "value": false}},
				},
				Except{2},
			},
			{
				"bool_case_2",
				Param{Filters: []map[string]any{
					{"name": "enabled", "op": "=", "value": true}},
				},
				Except{2},
			},

			{
				"bool_case_3",
				Param{Filters: []map[string]any{
					{"name": "enabled", "op": "=", "value": "xxxx"}},
				},
				Except{0},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data := map[string]any{
					"pos":     0,
					"limit":   5,
					"keyword": tt.params.Keyword,
					"filters": tt.params.Filters,
				}

				b, _ := json.Marshal(data)
				req := httptest.NewRequest(http.MethodPost, "/user/query", bytes.NewReader(b))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)

				var res QueryResult[User]
				err := json.Unmarshal(w.Body.Bytes(), &res)
				assert.Nil(t, err)
				assert.Equal(t, tt.expect.Num, res.TotalCount)
			})
		}

	}
}

func TestObjectOrder(t *testing.T) {
	type User struct {
		UUID string `json:"uid" gorm:"primarykey"`
		Name string `json:"name" gorm:"size:100"`
		Age  int
	}

	db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
	db.AutoMigrate(User{})

	r := gin.Default()
	webobject := WebObject[User]{
		Orders: []string{"ID", "Name", "Age"},
		GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return db.Debug()
		},
		Init: func(ctx *gin.Context, vptr *User) {},
	}
	err := webobject.RegisterObject(r)
	assert.Nil(t, err)

	// Mock data
	{
		db.Create(&User{UUID: "aaa", Name: "alice", Age: 9})
		db.Create(&User{UUID: "bbb", Name: "bob", Age: 10})
		db.Create(&User{UUID: "ccc", Name: "foo", Age: 13})
		db.Create(&User{UUID: "ddd", Name: "zoom", Age: 15})
	}
	// Query
	{
		type Param struct {
			Keyword string
			Orders  []map[string]any
		}
		type Except struct {
			ID string
		}
		tests := []struct {
			name   string
			params Param
			expect Except
		}{
			// {"base_case_1",
			// 	Param{Orders: []map[string]any{
			// 		{"name": "uid", "op": "desc"},
			// 	}},
			// 	Except{"aaa"},
			// },
			// {"base_case_2",
			// 	Param{Orders: []map[string]any{
			// 		{"name": "uid", "op": "asc"},
			// 	}},
			// 	Except{"aaa"},
			// },
			{"base_case_3",
				Param{Orders: []map[string]any{
					{"name": "Age", "op": "asc"},
				}},
				Except{"aaa"},
			},
			{"base_case_4",
				Param{Orders: []map[string]any{
					{"name": "Age", "op": "desc"},
				}},
				Except{"ddd"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data := map[string]any{
					"pos":    0,
					"limit":  5,
					"orders": tt.params.Orders,
				}

				b, _ := json.Marshal(data)
				req := httptest.NewRequest(http.MethodPost, "/user/query", bytes.NewReader(b))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)

				var res QueryResult[User]
				err := json.Unmarshal(w.Body.Bytes(), &res)
				assert.Nil(t, err)
				assert.Equal(t, tt.expect.ID, res.Items[0].UUID)
			})
		}

	}
}

// TODO:
func TestObjectEdit(t *testing.T) {
	type User struct {
		UUID       string    `json:"uid" gorm:"primarykey"`
		Name       string    `json:"name" gorm:"size:100"`
		Age        int       `json:"age"`
		Enabled    bool      `json:"enabled"`
		Birthday   time.Time `json:"birthday"`
		CannotEdit string    `json:"cannotEdit"`
	}

	// Query
	{
		type Param struct {
			ID   uint
			Data map[string]any
		}
		type Except struct {
			Code int
		}
		tests := []struct {
			name   string
			params Param
			expect Except
		}{
			{"base_case_1",
				Param{1, map[string]any{
					"name": "hhhhh",
					"age":  12,
				}},
				Except{http.StatusOK},
			},
			{"base_case_2",
				Param{1, map[string]any{
					"name": true,
					"age":  "12",
				}},
				Except{http.StatusBadRequest},
			},
			{"base_case_3",
				Param{1, map[string]any{
					"name": 11,
				}},
				Except{http.StatusBadRequest},
			},
			{"base_case_4",
				Param{1, map[string]any{
					"enabled": true,
				}},
				Except{http.StatusOK},
			},
			{"bad_case_1",
				Param{1, map[string]any{}},
				Except{http.StatusBadRequest},
			},
			{"bad_case_2",
				Param{1, map[string]any{
					"xxxxxx": "xxxxxx",
				}},
				Except{http.StatusBadRequest},
			},
			{"bad_case_3",
				Param{1, map[string]any{
					"cannotEdit": "xxxxxx",
				}},
				Except{http.StatusBadRequest},
			},
			// TODO:
			{"bad_case_4",
				Param{1, map[string]any{
					"name": nil,
				}},
				Except{http.StatusBadRequest},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
				db.AutoMigrate(User{})

				r := gin.Default()
				webobject := WebObject[User]{
					Editables: []string{"Name", "Age", "Enabled"},
					GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
						return db.Debug()
					},
					Init: func(ctx *gin.Context, u *User) {},
				}
				err := webobject.RegisterObject(r)
				assert.Nil(t, err)

				// Mock data
				{
					db.Create(&User{UUID: "aaa", Name: "alice", Age: 9})
				}

				b, _ := json.Marshal(tt.params.Data)
				req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/user/%d", tt.params.ID), bytes.NewReader(b))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				assert.Equal(t, tt.expect.Code, w.Result().StatusCode)
			})
		}
	}
}

func TestObjectNoFieldEdit(t *testing.T) {
	type User struct {
		ID       uint      `json:"uid" gorm:"primarykey"`
		Name     string    `json:"name" gorm:"size:100"`
		Age      int       `json:"age"`
		Enabled  bool      `json:"enabled"`
		Birthday time.Time `json:"birthday"`
	}

	db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
	db.AutoMigrate(User{})

	r := gin.Default()
	webobject := WebObject[User]{
		Editables: []string{},
		GetDB:     func(ctx *gin.Context, isCreate bool) *gorm.DB { return db },
	}
	err := webobject.RegisterObject(r)
	assert.Nil(t, err)

	db.Create(&User{ID: 1, Name: "alice", Age: 9})

	var data = map[string]any{
		"name":    "updatename",
		"age":     11,
		"enabled": true,
		"birthay": "2022-02-02 11:11:11",
	}
	b, _ := json.Marshal(data)
	req := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/user/%d", 1), bytes.NewReader(b))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}

func TestObjectRegister(t *testing.T) {
	type User struct {
		UUID     string    `json:"uid" gorm:"primarykey"`
		Name     string    `json:"name" gorm:"size:100"`
		Age      int       `json:"age"`
		Enabled  bool      `json:"enabled"`
		Birthday time.Time `json:"birthday"`
	}

	{
		type Param struct {
			Filterable []string
			Filters    []map[string]any
		}
		type Except struct {
			Total int
		}
		tests := []struct {
			name   string
			params Param
			expect Except
		}{
			{"filter by name and name is filterable",
				Param{
					[]string{"Name"},
					[]map[string]any{{"name": "name", "op": "=", "value": "alice"}},
				},
				Except{1},
			},
			{"filter by name but name is not filterable",
				Param{
					[]string{"Age"},
					[]map[string]any{{"name": "name", "op": "=", "value": "alice"}},
				},
				Except{4},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
				db.AutoMigrate(User{})

				r := gin.Default()
				webobject := WebObject[User]{
					Filters: tt.params.Filterable,
					GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
						return db.Debug()
					},
					Init: func(ctx *gin.Context, vptr *User) {},
				}
				err := webobject.RegisterObject(r)
				assert.Nil(t, err)

				// Mock data
				{
					db.Create(&User{UUID: "1", Name: "alice", Age: 9})
					db.Create(&User{UUID: "2", Name: "bob", Age: 10})
					db.Create(&User{UUID: "3", Name: "clash", Age: 11})
					db.Create(&User{UUID: "4", Name: "duck", Age: 12})
				}

				data := map[string]any{
					"pos":     0,
					"limit":   5,
					"filters": tt.params.Filters,
				}

				b, _ := json.Marshal(data)
				req := httptest.NewRequest(http.MethodPost, "/user/query", bytes.NewReader(b))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)

				var res QueryResult[User]
				json.Unmarshal(w.Body.Bytes(), &res)
				assert.Equal(t, tt.expect.Total, res.TotalCount)
			})
		}
	}
}

func TestBatchDelete(t *testing.T) {
	type User struct {
		UUID     uint      `json:"uid" gorm:"primarykey"`
		Name     string    `json:"name" gorm:"size:100"`
		Age      int       `json:"age"`
		Enabled  bool      `json:"enabled"`
		Birthday time.Time `json:"birthday"`
	}

	db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
	db.AutoMigrate(User{})

	r := gin.Default()
	webobject := WebObject[User]{
		GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB { return db },
	}
	err := webobject.RegisterObject(r)
	assert.Nil(t, err)

	db.Create(&User{UUID: 1, Name: "alice", Age: 9})
	db.Create(&User{UUID: 2, Name: "bob", Age: 10})
	db.Create(&User{UUID: 3, Name: "charlie", Age: 11})
	db.Create(&User{UUID: 4, Name: "dave", Age: 12})

	var data = map[string]any{
		"delete": []string{"1", "2"},
	}
	b, _ := json.Marshal(data)
	req := httptest.NewRequest(http.MethodPost, "/user/batch", bytes.NewReader(b))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	req = httptest.NewRequest(http.MethodPost, "/user/query", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var res QueryResult[User]
	json.Unmarshal(w.Body.Bytes(), &res)
	assert.Equal(t, 2, res.TotalCount)
}
