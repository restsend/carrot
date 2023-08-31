package apidocs

import (
	"reflect"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/restsend/carrot"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestParseFields(t *testing.T) {

	type Person struct {
		ID        string    `json:"id" comment:"id card no"`
		CreatedAt time.Time `json:"createdAt"`
	}

	type Woman struct {
		Person
	}

	type Man struct {
		Person               // embeded
		Name       string    `json:"name" binding:"required" comment:"username"`
		Age        int       `json:"age" comment:"user age"`
		Hobbies    []string  `json:"hobbies" comment:"hobbies"`
		Nums       []int     `json:"nums"`
		Display    *string   `json:"display" comment:"display name"`
		Woman      Woman     `json:"woman" comment:"women info"`
		OtherWoman *Woman    `json:"otherWoman"`
		Lovers     []Woman   `json:"lovers"`
		Birthday   time.Time `json:"birthday" comment:"birthday"`
	}

	man := Man{Name: "bob"}

	reflectType := reflect.TypeOf(man)
	rpcField := parseDocField(reflectType, "", nil)
	// assert.Equal(t, 11, reflectType.NumField())
	// ID
	assert.Equal(t, "string", rpcField.Fields[0].Type)
	// CreatedAt
	assert.Equal(t, "date", rpcField.Fields[1].Type)
	// Name
	assert.True(t, rpcField.Fields[2].Required)
	assert.Equal(t, rpcField.Fields[2].Desc, "username")
	// Age
	assert.Equal(t, "int", rpcField.Fields[3].Type)
	// Hobbies
	assert.Equal(t, "hobbies", rpcField.Fields[4].Name)
	assert.Equal(t, "string", rpcField.Fields[4].Type)
	assert.True(t, rpcField.Fields[4].IsArray)
	// Nums
	assert.Equal(t, "int", rpcField.Fields[5].Type)
	assert.True(t, rpcField.Fields[5].IsArray)
	// Display
	assert.Equal(t, "string", rpcField.Fields[6].Type)
	assert.True(t, rpcField.Fields[6].CanNull)
	// Woman
	assert.Equal(t, "object", rpcField.Fields[7].Type)
	assert.Equal(t, 2, len(rpcField.Fields[7].Fields))
	assert.Equal(t, "id", rpcField.Fields[7].Fields[0].Name)
	assert.Equal(t, "string", rpcField.Fields[7].Fields[0].Type)
	assert.Equal(t, "createdAt", rpcField.Fields[7].Fields[1].Name)
	assert.Equal(t, "date", rpcField.Fields[7].Fields[1].Type)
	// OtherWoman
	assert.Equal(t, "object", rpcField.Fields[8].Type)
	assert.True(t, rpcField.Fields[8].CanNull)
	assert.Equal(t, 2, len(rpcField.Fields[8].Fields))
	// Lovers
	assert.Equal(t, "object", rpcField.Fields[9].Type)
	assert.True(t, rpcField.Fields[9].IsArray)
	assert.Equal(t, 2, len(rpcField.Fields[9].Fields))
	// Birthday
	assert.Equal(t, "date", rpcField.Fields[10].Type)
}

func TestBuildRpcDefine(t *testing.T) {
	type demoObject struct {
		UUID     string   `json:"id" gorm:"primarykey;size:20"`
		Children []string `json:"children"`
	}
	o := carrot.WebObject{
		Model: &demoObject{},
		Name:  "DemoObject",
		GetDB: func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return nil
		},
	}
	err := o.Build()
	assert.Nil(t, err)

	//define := GetWebObjectDocDefine("", &o)
	//assert.Equal(t, len(define.Defines), 5)
	//assert.Equal(t, define.Name, "DemoObject")
}
