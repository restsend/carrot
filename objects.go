package carrot

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	DefaultQueryLimit = 50
)
const (
	FieldTypeNull     = "null"
	FieldTypeInt      = "int"
	FieldTypeBoolean  = "boolean"
	FieldTypeFloat    = "float"
	FieldTypeDatetime = "datetime"
	FieldTypeString   = "string"
	FieldTypeArray    = "array"
	FieldTypeMap      = "map"
	FieldTypeObject   = "object"
)

const (
	FilterOpEqual          = "="
	FilterOpNotEqual       = "<>"
	FilterOpIn             = "in"
	FilterOpNotIn          = "not_in"
	FilterOpGreater        = ">"
	FilterOpGreaterOrEqual = ">="
	FilterOpLess           = "<"
	FilterOpLessOrEqual    = "<="
	OrderOpDesc            = "desc"
	OrderOpAsc             = "asc"
)

type GetDB func(ctx *gin.Context, isCreate bool) *gorm.DB
type PrepareModel func(ctx *gin.Context, vptr interface{})
type PrepareQuery func(ctx *gin.Context, obj *WebObject) (*gorm.DB, *QueryForm, error)

type QueryView struct {
	Name    string
	Prepare PrepareQuery
}

type WebObject struct {
	Model     interface{}
	Group     string
	Name      string
	Editables []string
	Filters   []string
	Orders    []string
	Searchs   []string
	GetDB     GetDB
	Init      PrepareModel
	Views     []QueryView

	primaryKeyType     reflect.Type
	primaryKeyName     string
	primaryKeyJsonName string
	tableName          string
	modelElem          reflect.Type
	jsonToFields       map[string]string
}

type Filter struct {
	Name        string      `json:"name"`
	Op          string      `json:"op"`
	Value       string      `json:"value"`
	targetValue interface{} `json:"-"`
}

type Order struct {
	Name string `json:"name"`
	Op   string `json:"op"`
}

type QueryForm struct {
	Pos          int      `json:"pos"`
	Limit        int      `json:"limit"`
	Keyword      string   `json:"keyword,omitempty"`
	Filters      []Filter `json:"filters,omitempty"`
	Orders       []Order  `json:"orders,omitempty"`
	searchFields []string `json:"-"`
}

type QueryResult struct {
	TotalCount int         `json:"total,omitempty"`
	Pos        int         `json:"pos,omitempty"`
	Limit      int         `json:"limit,omitempty"`
	Keyword    string      `json:"keyword,omitempty"`
	Items      interface{} `json:"items,omitempty"`
}

func (f *Filter) GetQuery() string {
	var op string
	switch f.Op {
	case FilterOpEqual:
		return f.Name
	case FilterOpNotEqual:
		op = "<>"
	case FilterOpIn:
		op = "IN"
	case FilterOpNotIn:
		op = "NOT IN"
	case FilterOpGreater:
		op = ">"
	case FilterOpGreaterOrEqual:
		op = ">="
	case FilterOpLess:
		op = "<"
	case FilterOpLessOrEqual:
		op = "<="
	}
	return fmt.Sprintf("%s %s ?", f.Name, op)
}

func (f *Filter) GetValue() interface{} {
	if f.targetValue == nil && f.Value != "" {
		return f.Value
	}
	return f.targetValue
}

func (f *Order) GetQuery() string {
	if f.Op == OrderOpDesc {
		return f.Name + " DESC"
	}
	return f.Name
}

func ConvertKey(dst reflect.Type, v interface{}) interface{} {
	if v == nil {
		return nil
	}

	src := reflect.TypeOf(v)
	if src.Kind() == dst.Kind() {
		return v
	}

	if src.Kind() == reflect.String {
		switch dst.Kind() {
		case reflect.Int:
			x, _ := strconv.ParseInt(v.(string), 10, 64)
			return int(x)
		case reflect.Int64:
			x, _ := strconv.ParseInt(v.(string), 10, 64)
			return x
		case reflect.Uint:
			x, _ := strconv.ParseUint(v.(string), 10, 64)
			return uint(x)
		case reflect.Uint64:
			x, _ := strconv.ParseUint(v.(string), 10, 64)
			return x
		case reflect.Bool:
			if v.(string) == "true" || v.(string) == "yes" {
				return true
			} else {
				return false
			}
		case reflect.Float32, reflect.Float64:
			x, _ := strconv.ParseFloat(v.(string), 64)
			return x
		}
	}
	return fmt.Sprintf("%v", v)
}

func QueryObjects(db *gorm.DB, obj *WebObject, form *QueryForm) (r QueryResult, err error) {
	tblName := db.NamingStrategy.TableName(obj.tableName)
	for _, v := range form.Filters {
		q := v.GetQuery()
		if q != "" {
			db = db.Where(fmt.Sprintf("%s.%s", tblName, v.GetQuery()), v.GetValue())
		}
	}

	for _, v := range form.Orders {
		q := v.GetQuery()
		if q != "" {
			db = db.Order(fmt.Sprintf("%s.%s", tblName, v.GetQuery()))
		}
	}

	if form.Keyword != "" && len(form.searchFields) > 0 {
		var query []string
		for _, v := range form.searchFields {
			query = append(query, fmt.Sprintf("`%s`.`%s` LIKE @keyword", tblName, v))
		}
		searchKey := strings.Join(query, " OR ")
		db = db.Where(searchKey, sql.Named("keyword", "%"+form.Keyword+"%"))
	}

	limit := DefaultQueryLimit
	if form.Limit > 0 && form.Limit < DefaultQueryLimit {
		limit = form.Limit
	}

	r.Pos = form.Pos
	r.Limit = limit
	r.Keyword = form.Keyword

	var c int64
	model := reflect.New(obj.modelElem).Interface()
	result := db.Model(model).Count(&c)
	if result.Error != nil {
		return r, result.Error
	}
	r.TotalCount = int(c)
	if c <= 0 {
		return r, nil
	}

	db = db.Offset(form.Pos).Limit(limit)
	items := reflect.New(reflect.SliceOf(obj.modelElem))
	result = db.Find(items.Interface())
	if result.Error != nil {
		return r, result.Error
	}
	r.Items = items.Elem().Interface()
	r.Pos += int(result.RowsAffected)
	return r, nil
}

func handleGetObject(c *gin.Context, obj *WebObject) {
	key := ConvertKey(obj.primaryKeyType, c.Param("key"))
	val := reflect.New(obj.modelElem).Interface()
	colName := obj.GetDB(c, false).NamingStrategy.ColumnName(obj.tableName, obj.primaryKeyName)
	result := obj.GetDB(c, false).Where(colName, key).Take(&val)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, val)
}

func handleCreateObject(c *gin.Context, obj *WebObject) {
	val := reflect.New(obj.modelElem).Interface()
	err := c.BindJSON(&val)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if obj.Init != nil {
		obj.Init(c, val)
	}
	result := obj.GetDB(c, true).Create(val)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, val)
}

func handleEditObject(c *gin.Context, obj *WebObject) {
	key := ConvertKey(obj.primaryKeyType, c.Param("key"))
	var inputVals map[string]interface{}
	err := c.BindJSON(&inputVals)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	db := obj.GetDB(c, false)
	namer := db.NamingStrategy

	var vals map[string]interface{} = map[string]interface{}{}
	keyName := namer.ColumnName(obj.tableName, obj.primaryKeyName)
	delete(inputVals, obj.primaryKeyJsonName) // remove primaryKey

	for k, v := range inputVals {
		if fname, ok := obj.jsonToFields[k]; ok {
			vals[fname] = v
		}
	}

	if len(obj.Editables) > 0 {
		stripVals := make(map[string]interface{})
		for _, k := range obj.Editables {
			if v, ok := vals[k]; ok {
				stripVals[k] = v
			}
		}
		vals = stripVals
	}

	if len(vals) <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "not changed"})
		return
	}
	model := reflect.New(obj.modelElem).Interface()
	result := db.Model(model).Where(keyName, key).UpdateColumns(vals)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, true)
}

func handleDeleteObject(c *gin.Context, obj *WebObject) {
	key := ConvertKey(obj.primaryKeyType, c.Param("key"))
	db := obj.GetDB(c, false)
	keyName := db.NamingStrategy.ColumnName(obj.tableName, obj.primaryKeyName)

	val := reflect.New(obj.modelElem).Interface()
	result := db.Where(keyName, key).Delete(val)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": result.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, true)
}

func DefaultPrepareQuery(c *gin.Context, obj *WebObject) (*gorm.DB, *QueryForm, error) {
	var form QueryForm
	if c.Request.ContentLength > 0 {
		if err := c.BindJSON(&form); err != nil {
			return nil, nil, err
		}
	}
	return obj.GetDB(c, false), &form, nil
}

func HandleQueryObject(c *gin.Context, obj *WebObject, prepareQuery PrepareQuery) {
	db, form, err := prepareQuery(c, obj)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	namer := db.NamingStrategy

	var filterFields map[string]struct{} = make(map[string]struct{})
	for _, k := range obj.Filters {
		filterFields[k] = struct{}{}
	}

	var stripFilters []Filter

	for i := 0; i < len(form.Filters); i++ {
		f := form.Filters[i]
		n, ok := obj.jsonToFields[f.Name]
		if !ok {
			continue
		}
		f.Name = n // replace to struct filed name
		if len(filterFields) != 0 {
			if _, ok := filterFields[f.Name]; !ok {
				continue
			}
		}
		f.Name = namer.ColumnName(obj.tableName, f.Name)
		fe, _ := obj.modelElem.FieldByName(n)
		f.targetValue = ConvertKey(fe.Type, f.Value)
		stripFilters = append(stripFilters, f)
	}
	form.Filters = stripFilters

	var orderFields map[string]struct{} = make(map[string]struct{})
	for _, k := range obj.Orders {
		orderFields[k] = struct{}{}
	}

	var stripOrders []Order

	for i := 0; i < len(form.Orders); i++ {
		f := form.Orders[i]
		n, ok := obj.jsonToFields[f.Name]
		if !ok {
			continue
		}
		f.Name = n // replace to struct filed name
		if len(orderFields) != 0 {
			if _, ok := orderFields[f.Name]; !ok {
				continue
			}
		}
		f.Name = namer.ColumnName(obj.tableName, f.Name)
		stripOrders = append(stripOrders, f)
	}
	form.Orders = stripOrders

	if form.Keyword != "" {
		form.searchFields = []string{}
		for _, v := range obj.Searchs {
			form.searchFields = append(form.searchFields, namer.ColumnName(obj.tableName, v))
		}
	}

	result, err := QueryObjects(db, obj, form)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func RegisterObject(r gin.IRoutes, obj *WebObject) {

	if err := obj.Build(); err != nil {
		log.Printf("[error] %v", err)
		return
	}

	p := filepath.Join(obj.Group, obj.Name)

	r.GET(filepath.Join(p, ":key"), func(c *gin.Context) {
		handleGetObject(c, obj)
	})

	//Create
	r.PUT(p, func(c *gin.Context) {
		handleCreateObject(c, obj)
	})
	//Edit
	r.PATCH(filepath.Join(p, ":key"), func(c *gin.Context) {
		handleEditObject(c, obj)
	})

	//Delete
	r.DELETE(filepath.Join(p, ":key"), func(c *gin.Context) {
		handleDeleteObject(c, obj)
	})
	// Query
	r.POST(filepath.Join(p, "query"), func(c *gin.Context) {
		HandleQueryObject(c, obj, DefaultPrepareQuery)
	})

	for i := 0; i < len(obj.Views); i++ {
		v := &obj.Views[i]
		r.POST(filepath.Join(p, v.Name), func(ctx *gin.Context) {
			f := v.Prepare
			if f == nil {
				f = DefaultPrepareQuery
			}
			HandleQueryObject(ctx, obj, v.Prepare)
		})
	}
}

func RegisterObjects(r gin.IRoutes, objs []WebObject) {
	for idx := range objs {
		obj := &objs[idx]
		RegisterObject(r, obj)
	}
}

func (obj *WebObject) parseFields(rt reflect.Type) {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)

		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			obj.parseFields(f.Type)
		}
		jsonTag := f.Tag.Get("json")
		if jsonTag == "" {
			obj.jsonToFields[f.Name] = f.Name
		} else if jsonTag != "-" {
			obj.jsonToFields[jsonTag] = f.Name
		}

		gormTag := f.Tag.Get("gorm")
		if gormTag == "" || gormTag == "-" {
			continue
		}

		if !strings.Contains(gormTag, "primarykey") {
			continue
		}
		if jsonTag == "" || jsonTag == "-" {
			obj.primaryKeyJsonName = f.Name
		} else {
			obj.primaryKeyJsonName = jsonTag
		}
		obj.primaryKeyName = f.Name
		obj.primaryKeyType = f.Type
	}
}

func (obj *WebObject) Build() error {
	obj.modelElem = reflect.TypeOf(obj.Model)
	if obj.modelElem.Kind() == reflect.Ptr {
		obj.modelElem = obj.modelElem.Elem()
	}

	obj.tableName = obj.modelElem.Name()
	obj.jsonToFields = make(map[string]string)

	if obj.GetDB == nil {
		obj.GetDB = func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return ctx.MustGet(DbField).(*gorm.DB)
		}
	}

	obj.parseFields(obj.modelElem)

	if obj.primaryKeyName == "" {
		return fmt.Errorf("%s not primaryKey", obj.Name)
	}

	if obj.Name == "" {
		obj.Name = strings.ToLower(obj.tableName)
	}
	return nil
}
