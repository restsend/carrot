package carrot

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
	r.Use(WithGormDB(db))
	webobject := WebObject{
		Model:       User{},
		Editables:   []string{"Name"},
		Filterables: []string{"Name"},
		Searchables: []string{"Name"},
	}
	err = webobject.RegisterObject(&r.RouterGroup)
	assert.Nil(t, err)

	// Create
	{
		b, _ := Marshal(User{Name: "add"})
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
		b, _ := Marshal(User{Name: "update", Age: 11})
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
		b, _ := Marshal(data)
		req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(b))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)

		var res QueryResult
		err := Unmarshal(w.Body.Bytes(), &res)
		assert.Nil(t, err)
		assert.Equal(t, 1, res.TotalCount)
		assert.Equal(t, "update", res.Items[0].(map[string]any)["Name"])
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

		b, _ := Marshal(map[string]any{"pos": 0, "limit": 5})
		req = httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(b))
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Result().StatusCode)
		log.Println(w.Body.String())

		var res QueryResult
		err := Unmarshal(w.Body.Bytes(), &res)
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
	r.Use(WithGormDB(db))
	webobject := WebObject{
		Model:       User{},
		Filterables: []string{"Name", "Age", "Birthday", "Enabled"},
		Searchables: []string{"Name"},

		BeforeRender: func(db *gorm.DB, c *gin.Context, obj any) (any, error) {
			return obj, nil
		},
	}
	err := webobject.RegisterObject(&r.RouterGroup)
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
					{"name": "Age", "op": ">=", "value": "10"},
					{"name": "Age", "op": "<=", "value": "12"}},
				},
				Except{2},
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
			{
				"like_case_1",
				Param{Filters: []map[string]any{
					{"name": "name", "op": "like", "value": "a"},
				}},
				Except{2},
			},
			{
				"like_case_2",
				Param{Filters: []map[string]any{
					{"name": "name", "op": "like", "value": "bo"},
					{"name": "name", "op": "like", "value": "b"},
				}},
				Except{1},
			},
			{
				"like_case_3",
				Param{Filters: []map[string]any{
					{"name": "Age", "op": "like", "value": "3"},
				}},
				Except{2},
			},
			{
				"not_int_case_1",
				Param{Filters: []map[string]any{
					{"name": "name", "op": "not_in", "value": []any{"alice", "bob", "foo"}},
				}},
				Except{1},
			},
			{
				"not_int_case_2",
				Param{Filters: []map[string]any{
					{"name": "Age", "op": "not_in", "value": []any{10, 13}},
				}},
				Except{0},
			},
			{
				"bad_case_1: for op not exist",
				Param{Filters: []map[string]any{
					{"name": "name", "op": "notexist", "value": "xxxx"},
				}},
				Except{4},
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

				b, _ := Marshal(data)
				req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(b))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)

				var res QueryResult
				err := Unmarshal(w.Body.Bytes(), &res)
				assert.Nil(t, err)
				assert.Equal(t, tt.expect.Num, res.TotalCount)
			})
		}

	}
}

func TestObjectOrder(t *testing.T) {
	type User struct {
		UUID      string    `json:"uid" gorm:"primarykey"`
		CreatedAt time.Time `json:"createdAt"`
		Name      string    `json:"name" gorm:"size:100"`
		Age       int
	}

	db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
	db.AutoMigrate(User{})

	r := gin.Default()
	r.Use(WithGormDB(db))
	webobject := WebObject{
		Model:      User{},
		Orderables: []string{"UUID", "Name", "Age", "CreatedAt"},
	}
	err := webobject.RegisterObject(&r.RouterGroup)
	assert.Nil(t, err)

	// Mock data
	{
		db.Create(&User{UUID: "aaa", Name: "alice", Age: 9, CreatedAt: time.Now()})
		db.Create(&User{UUID: "bbb", Name: "bob", Age: 10, CreatedAt: time.Now().Add(time.Second * 5)})
		db.Create(&User{UUID: "ccc", Name: "foo", Age: 13, CreatedAt: time.Now().Add(time.Second * 10)})
		db.Create(&User{UUID: "ddd", Name: "zoom", Age: 15, CreatedAt: time.Now().Add(time.Second * 15)})
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
			{"base_case_1:name_desc",
				Param{Orders: []map[string]any{
					{"name": "name", "op": "desc"},
				}},
				Except{"ddd"},
			},
			{"base_case_2:name_asc",
				Param{Orders: []map[string]any{
					{"name": "name", "op": "asc"},
				}},
				Except{"aaa"},
			},
			{"base_case_3:nil",
				Param{Orders: nil},
				Except{"aaa"},
			},
			{"base_case_4:age_asc",
				Param{Orders: []map[string]any{
					{"name": "Age", "op": "asc"},
				}},
				Except{"aaa"},
			},
			{"base_case_5:age_desc",
				Param{Orders: []map[string]any{
					{"name": "Age", "op": "desc"},
				}},
				Except{"ddd"},
			},
			{"base_case_6:createdAt_asc",
				Param{Orders: []map[string]any{
					{"name": "createdAt", "op": "asc"},
				}},
				Except{"aaa"},
			},
			{"base_case_5:createdAt_desc",
				Param{Orders: []map[string]any{
					{"name": "createdAt", "op": "desc"},
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

				b, _ := Marshal(data)
				req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(b))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)

				var res QueryResult
				err := Unmarshal(w.Body.Bytes(), &res)
				assert.Nil(t, err)
				assert.Equal(t, tt.expect.ID, res.Items[0].(map[string]any)["uid"])
			})
		}

	}
}

// TODO:
func TestObjectEdit(t *testing.T) {
	type User struct {
		UUID       string     `json:"uid" gorm:"primarykey"`
		Name       string     `json:"name" gorm:"size:100"`
		Age        int        `json:"age"`
		Enabled    bool       `json:"enabled"`
		Birthday   time.Time  `json:"birthday"`
		CannotEdit string     `json:"cannotEdit"`
		PtrTime    *time.Time `json:"ptrTime"`
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
			{"time_case_1",
				Param{
					1, map[string]any{
						"birthday": "2023-03-13T10:27:11.9802049+08:00",
					}},
				Except{http.StatusOK},
			},
			{"time_case_2",
				Param{
					1, map[string]any{
						"birthday": nil,
					}},
				Except{http.StatusBadRequest},
			},
			{"ptr_case_1",
				Param{
					1, map[string]any{
						"ptrTime": "2023-03-16T15:03:04.21432577Z",
					}},
				Except{http.StatusOK},
			},
			{"ptr_case_2",
				Param{
					1, map[string]any{
						"ptrTime": nil,
					}},
				Except{http.StatusBadRequest},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
				db.AutoMigrate(User{})

				r := gin.Default()
				r.Use(WithGormDB(db))
				webobject := WebObject{
					Model:     User{},
					Editables: []string{"Name", "Age", "Enabled", "Birthday", "PtrTime"},
				}
				err := webobject.RegisterObject(&r.RouterGroup)
				assert.Nil(t, err)

				// Mock data
				{
					db.Create(&User{UUID: "aaa", Name: "alice", Age: 9})
				}

				b, _ := Marshal(tt.params.Data)
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
	r.Use(WithGormDB(db))
	webobject := WebObject{
		Model:     User{},
		Editables: []string{},
	}
	err := webobject.RegisterObject(&r.RouterGroup)
	assert.Nil(t, err)

	db.Create(&User{ID: 1, Name: "alice", Age: 9})

	var data = map[string]any{
		"name":    "updatename",
		"age":     11,
		"enabled": true,
		"birthay": "2022-02-02 11:11:11",
	}
	b, _ := Marshal(data)
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
			{"filter by time type",
				Param{
					[]string{"Birthday"},
					[]map[string]any{{"name": "birthday", "op": ">=", "value": time.Now().Add(time.Hour * -1).Format(time.RFC3339)}},
				},
				Except{1},
			},
			{"filter by time type between",
				Param{
					[]string{"Birthday"},
					[]map[string]any{{"name": "birthday", "op": "between", "value": []any{time.Now().Add(time.Hour * -1).Format(time.RFC3339), time.Now().Format(time.RFC3339)}}},
				},
				Except{1},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {

				db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
				db.AutoMigrate(User{})

				r := gin.Default()
				r.Use(WithGormDB(db))
				webobject := WebObject{
					Model:       User{},
					Filterables: tt.params.Filterable,
				}
				err := webobject.RegisterObject(&r.RouterGroup)
				assert.Nil(t, err)

				// Mock data
				{
					db.Create(&User{UUID: "1", Name: "alice", Age: 9})
					db.Create(&User{UUID: "2", Name: "bob", Age: 10})
					db.Create(&User{UUID: "3", Name: "clash", Age: 11})
					db.Create(&User{UUID: "4", Name: "duck", Age: 12, Birthday: time.Now().Add(time.Minute * -10)})
				}

				data := map[string]any{
					"pos":     0,
					"limit":   5,
					"filters": tt.params.Filters,
				}

				b, _ := Marshal(data)
				req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewReader(b))
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				assert.Equal(t, http.StatusOK, w.Result().StatusCode)

				var res QueryResult
				Unmarshal(w.Body.Bytes(), &res)
				assert.Equal(t, tt.expect.Total, res.TotalCount)
			})
		}
	}
}

type UnittestUser struct {
	ID   uint   `json:"id" gorm:"primarykey"`
	Name string `json:"name" gorm:"size:100"`
	Age  int    `json:"age"`
}

func initHookTest(t *testing.T) (TestClient, *gorm.DB) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
	db.AutoMigrate(UnittestUser{})

	db.Create(&UnittestUser{ID: 1, Name: "alice", Age: 9})
	db.Create(&UnittestUser{ID: 2, Name: "bob", Age: 10})
	db.Create(&UnittestUser{ID: 3, Name: "clash", Age: 11})

	webobject := WebObject{
		Name:        "user",
		Model:       UnittestUser{},
		Editables:   []string{"Name"},
		Filterables: []string{"Name, Age"},
		Searchables: []string{"Name"},
		GetDB: func(c *gin.Context, isCreate bool) *gorm.DB {
			return db
		},
		BeforeCreate: func(db *gorm.DB, ctx *gin.Context, vptr any) error {
			user := (vptr).(*UnittestUser)
			if user.Name == "dangerous" {
				return errors.New("alice is not allowed to create")
			}
			return nil
		},
		BeforeRender: func(db *gorm.DB, ctx *gin.Context, vptr any) (any, error) {
			user := (vptr).(*UnittestUser)
			if user.Name != "alice" {
				user.Age = 99
			}
			return vptr, nil
		},
		BeforeDelete: func(db *gorm.DB, ctx *gin.Context, vptr any) error {
			user := (vptr).(*UnittestUser)
			if user.Name == "alice" {
				return errors.New("alice is not allowed to delete")
			}
			return nil
		},
		BeforeUpdate: func(db *gorm.DB, ctx *gin.Context, vptr any, vals map[string]any) error {
			user := (vptr).(*UnittestUser)
			if user.Name == "alice" {
				return errors.New("alice is not allowed to update")
			}
			if vals["name"] == "dangerous" {
				return errors.New("this name is not allowed to update")
			}
			return nil
		},
	}

	r := gin.Default()
	err := webobject.RegisterObject(&r.RouterGroup)
	assert.Nil(t, err)

	return *NewTestClient(r), db
}

func TestOnRender(t *testing.T) {
	c, _ := initHookTest(t)

	var res QueryResult
	err := c.CallPost("/user", nil, &res)

	assert.Nil(t, err)
	assert.Equal(t, 3, res.TotalCount)
	assert.Equal(t, float64(9), res.Items[0].(map[string]any)["age"])
	assert.Equal(t, float64(99), res.Items[1].(map[string]any)["age"])
	assert.Equal(t, float64(99), res.Items[2].(map[string]any)["age"])
}

func TestOnDelete(t *testing.T) {
	c, _ := initHookTest(t)

	err := c.CallDelete("/user/1", nil, nil)
	assert.NotNil(t, err)

	err = c.CallDelete("/user/2", nil, nil)
	assert.Nil(t, err)
}

func TestOnCreate(t *testing.T) {
	c, _ := initHookTest(t)

	err := c.CallPut("/user", UnittestUser{Name: "dangerous"}, nil)
	assert.NotNil(t, err) // alice is not allowed to create

	err = c.CallPut("/user", UnittestUser{Name: "notdangerous"}, nil)
	assert.Nil(t, err)
}

func TestOnUpdate(t *testing.T) {
	c, _ := initHookTest(t)

	err := c.CallPatch("/user/1", map[string]any{"name": "notdangerous"}, nil)
	assert.NotNil(t, err) // alice is not allowed to update

	err = c.CallPatch("/user/2", map[string]any{"name": "dangerous"}, nil)
	assert.NotNil(t, err) // this name is not allowed to update

	err = c.CallPatch("/user/2", map[string]any{"name": "notdangerous"}, nil)
	assert.Nil(t, err)
}

func TestQueryViews(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
	db.AutoMigrate(UnittestUser{})

	r := gin.Default()
	r.Use(WithGormDB(db))

	webobject := WebObject{
		Name:        "user",
		Model:       UnittestUser{},
		Editables:   []string{"Name"},
		Filterables: []string{"Name, Age"},
		Searchables: []string{"Name"},
		Views: []QueryView{
			{
				Path:   "names",
				Method: http.MethodGet,
				Prepare: func(db *gorm.DB, ctx *gin.Context) (*gorm.DB, *QueryForm, error) {
					return db, &QueryForm{Limit: -1, ViewFields: []string{"ID", "Name"}}, nil
				},
			},
		},
	}
	err := webobject.RegisterObject(&r.RouterGroup)
	assert.Nil(t, err)

	// create 200 users
	var user [200]UnittestUser
	for i := 0; i < len(user); i++ {
		user[i] = UnittestUser{Name: fmt.Sprintf("user-%d", i), Age: i}
	}
	db.CreateInBatches(&user, len(user))

	client := NewTestClient(r)
	var result QueryResult
	err = client.CallGet("/user/names", nil, &result)
	assert.Nil(t, err)
	assert.Equal(t, 200, result.TotalCount)
	assert.Equal(t, 200, len(result.Items))
	assert.Equal(t, float64(0), result.Items[10].(map[string]any)["age"])
	assert.NotZero(t, result.Items[10].(map[string]any)["id"])
}

func TestBeforeRender(t *testing.T) {
	db, _ := gorm.Open(sqlite.Open("file::memory:"), nil)
	db.AutoMigrate(UnittestUser{})

	r := gin.Default()
	r.Use(WithGormDB(db))

	webobject := WebObject{
		Name:        "user",
		Model:       UnittestUser{},
		Editables:   []string{"Name"},
		Filterables: []string{"Name, Age"},
		Searchables: []string{"Name"},
		BeforeRender: func(db *gorm.DB, ctx *gin.Context, vptr any) (any, error) {
			ctx.Redirect(http.StatusMovedPermanently, "/not_found")
			return nil, nil
		},
	}
	err := webobject.RegisterObject(&r.RouterGroup)
	assert.Nil(t, err)

	client := NewTestClient(r)
	user := UnittestUser{
		ID:   1,
		Name: fmt.Sprintf("user-%d", 1),
		Age:  1,
	}
	db.Create(&user)

	err = client.CallGet("/user/1", nil, nil)
	assert.Contains(t, err.Error(), "Moved Permanently")
}
