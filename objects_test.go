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
	r, err := QueryObjects(db, reflect.TypeOf(bob).Elem(), &form)
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
	u := MockUser{
		Name: "user_1",
		Age:  10,
	}
	db.Create(&u)

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
			Init: func(ctx *gin.Context, vptr interface{}) {
			},
		},
	})
	client := NewTestClient(r)
	{
		user2 := MockUser{
			Name: "user_2",
			Age:  11,
		}

		body, _ := json.Marshal(&user2)
		req, _ := http.NewRequest(http.MethodPut, "/muser", bytes.NewReader(body))
		// Create
		w := client.SendReq("/muser", req)
		assert.Equal(t, w.Code, http.StatusOK)

		var u2 MockUser
		err = json.Unmarshal(w.Body.Bytes(), &u2)
		assert.Nil(t, err)
		assert.Equal(t, u2.Name, "user_2")

		// Get after create
		w = client.Get(fmt.Sprintf("/muser/%d", u2.ID))
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
		err = client.Call("/muser/query", &form, &result)
		assert.Nil(t, err)
		assert.Equal(t, result.TotalCount, 1)
		assert.Equal(t, result.Pos, 1)

		//Delete
		req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("/muser/%d", u2.ID), nil)
		w = client.SendReq(fmt.Sprintf("/muser/%d", u2.ID), req)
		assert.Equal(t, w.Code, http.StatusOK)

		// Will not found
		w = client.Get(fmt.Sprintf("/muser/%d", u2.ID))
		assert.Equal(t, w.Code, http.StatusNotFound)
	}
}
