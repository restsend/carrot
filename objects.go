package carrot

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	DefaultQueryLimit = 50
)

// const (
// 	FieldTypeNull     = "null"
// 	FieldTypeInt      = "int"
// 	FieldTypeBoolean  = "boolean"
// 	FieldTypeFloat    = "float"
// 	FieldTypeDatetime = "datetime"
// 	FieldTypeString   = "string"
// 	FieldTypeArray    = "array"
// 	FieldTypeMap      = "map"
// 	FieldTypeObject   = "object"
// )

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

const (
	GET    = 1 << 1
	CREATE = 1 << 2
	EDIT   = 1 << 3
	DELETE = 1 << 4
	QUERY  = 1 << 5
)

type GetDB func(ctx *gin.Context, isCreate bool) *gorm.DB
type PrepareModel[T any] func(ctx *gin.Context, vptr *T)
type PrepareQuery func(ctx *gin.Context) (*QueryForm, error)

// TODO:
type QueryView struct {
	Name    string
	Prepare PrepareQuery
}

type WebObject[T any] struct {
	Model        T
	Group        string
	Name         string
	Editables    []string
	Filters      []string
	Orders       []string
	Searchs      []string
	GetDB        GetDB           // TODO: 抽取，解耦
	Init         PrepareModel[T] // How to create a new object
	Views        []QueryView
	AllowMethods int

	PrimaryKeyName     string
	PrimaryKeyJsonName string
	tableName          string

	// Map json tag to struct field name. such as:
	// UUID string `json:"id"` => {"id" : "UUID"}
	jsonToFields map[string]string
	// Map json tag to field type. such as:
	// UUID string `json:"id"` => {"id": string}
	jsonToKinds map[string]reflect.Kind
}

type Filter struct {
	Name  string `json:"name"`
	Op    string `json:"op"`
	Value any    `json:"value"`
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
	searchFields []string `json:"-"` // for Keyword
}

type QueryResult[T any] struct {
	TotalCount int    `json:"total,omitempty"`
	Pos        int    `json:"pos,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Keyword    string `json:"keyword,omitempty"`
	Items      []T    `json:"items"`
}

// GetQuery return the combined filter SQL statement.
// such as "age >= ?", "name IN ?".
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

// GetValue return the target value of the filter SQL statement.
func (f *Filter) GetValue() any {
	return f.Value
	// if f.targetValue == nil && f.Value != "" {
	// 	return f.Value
	// }
	// if f.Op != FilterOpIn && f.Op != FilterOpNotIn {
	// 	// return f.targetValue
	// 	return f.Value
	// }
	// var arrValues []any
	// err := json.Unmarshal([]byte(f.Value), &arrValues)
	// if err == nil {
	// 	return arrValues
	// }
	// return f.Value
}

// GetQuery return the combined order SQL statement.
// such as "id DESC".
func (f *Order) GetQuery() string {
	if f.Op == OrderOpDesc {
		return f.Name + " DESC"
	}
	return f.Name
}

func (obj *WebObject[T]) RegisterObject(r gin.IRoutes) error {
	if err := obj.Build(); err != nil {
		return err
	}

	p := filepath.Join(obj.Group, obj.Name)
	allowMethods := obj.AllowMethods
	if allowMethods == 0 {
		allowMethods = GET | CREATE | EDIT | DELETE | QUERY
	}

	if allowMethods&GET != 0 {
		r.GET(filepath.Join(p, ":key"), func(c *gin.Context) {
			handleGetObject(c, obj)
		})
	}
	if allowMethods&CREATE != 0 {
		r.PUT(p, func(c *gin.Context) {
			handleCreateObject(c, obj)
		})
	}
	if allowMethods&EDIT != 0 {
		r.PATCH(filepath.Join(p, ":key"), func(c *gin.Context) {
			handleEditObject(c, obj)
		})
	}

	if allowMethods&DELETE != 0 {
		r.DELETE(filepath.Join(p, ":key"), func(c *gin.Context) {
			handleDeleteObject(c, obj)
		})
	}

	if allowMethods&QUERY != 0 {
		r.POST(filepath.Join(p, "query"), func(c *gin.Context) {
			handleQueryObject(c, obj, DefaultPrepareQuery)
		})
	}

	for i := 0; i < len(obj.Views); i++ {
		v := &obj.Views[i]
		r.POST(filepath.Join(p, v.Name), func(ctx *gin.Context) {
			f := v.Prepare
			if f == nil {
				f = DefaultPrepareQuery
			}
			handleQueryObject(ctx, obj, v.Prepare)
		})
	}
	return nil
}

func RegisterObjects[T any](r gin.IRoutes, objs []WebObject[T]) {
	for idx := range objs {
		obj := &objs[idx]
		err := obj.RegisterObject(r)
		if err != nil {
			log.Printf("RegisterObject [%s] fail %v\n", obj.Name, err)
		}
	}
}

// Build fill the properties of obj.
func (obj *WebObject[T]) Build() error {
	var t T
	rt := reflect.TypeOf(t)

	obj.tableName = rt.Name()

	if obj.Name == "" {
		obj.Name = strings.ToLower(obj.tableName)
	}

	obj.jsonToFields = make(map[string]string)
	obj.jsonToKinds = make(map[string]reflect.Kind)
	obj.parseFields(rt)

	if obj.PrimaryKeyName == "" {
		return fmt.Errorf("%s not has primaryKey", obj.Name)
	}

	// TODO: 解耦，让 objects 可以作为工具类单独使用
	if obj.GetDB == nil {
		obj.GetDB = func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return ctx.MustGet(DbField).(*gorm.DB)
		}
	}

	return nil
}

// parseFields parse the following properties according to struct tag:
// - jsonToFields, jsonToKinds, primaryKeyName, primaryKeyJsonName
func (obj *WebObject[T]) parseFields(rt reflect.Type) {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)

		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			obj.parseFields(f.Type)
		}

		jsonTag := f.Tag.Get("json")
		if jsonTag == "" {
			obj.jsonToFields[f.Name] = f.Name
			obj.jsonToKinds[f.Name] = f.Type.Kind()
		} else if jsonTag != "-" {
			obj.jsonToFields[jsonTag] = f.Name
			obj.jsonToKinds[jsonTag] = f.Type.Kind()
		}

		gormTag := f.Tag.Get("gorm")
		if gormTag == "" || gormTag == "-" {
			continue
		}

		if !strings.Contains(gormTag, "primarykey") &&
			!strings.Contains(gormTag, "primaryKey") {
			continue
		}

		obj.PrimaryKeyName = f.Name
		if jsonTag == "" || jsonTag == "-" {
			obj.PrimaryKeyJsonName = f.Name
		} else {
			obj.PrimaryKeyJsonName = jsonTag
		}
	}
}

func handleGetObject[T any](c *gin.Context, obj *WebObject[T]) {
	key := c.Param("key")
	db := obj.GetDB(c, false)

	// the real name of the primaryKey column
	pkColName := db.NamingStrategy.ColumnName(obj.tableName, obj.PrimaryKeyName)

	var val T
	result := db.Where(pkColName, key).Take(&val)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	c.JSON(http.StatusOK, val)
}

func handleCreateObject[T any](c *gin.Context, obj *WebObject[T]) {
	var val *T

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

func handleEditObject[T any](c *gin.Context, obj *WebObject[T]) {
	key := c.Param("key")

	var inputVals map[string]any
	err := c.BindJSON(&inputVals)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := obj.GetDB(c, false)

	var vals map[string]any = map[string]any{}

	// can't edit primaryKey
	delete(inputVals, obj.PrimaryKeyJsonName)

	for k, v := range inputVals {
		// Check the kind to be edited.
		kind, ok := obj.jsonToKinds[k]
		if !ok {
			continue
		}

		fname, ok := obj.jsonToFields[k]
		if !ok {
			continue
		}

		if !checkType(kind, reflect.TypeOf(v).Kind()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s type not match", fname)})
			return
		}

		vals[fname] = v
	}

	if len(obj.Editables) > 0 {
		stripVals := make(map[string]any)
		for _, k := range obj.Editables {
			if v, ok := vals[k]; ok {
				stripVals[k] = v
			}
		}
		vals = stripVals
	}

	if len(vals) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "not changed"})
		return
	}

	var model T
	pkColName := db.NamingStrategy.ColumnName(obj.tableName, obj.PrimaryKeyName)
	result := db.Model(model).Where(pkColName, key).UpdateColumns(vals)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, true)
}

func handleDeleteObject[T any](c *gin.Context, obj *WebObject[T]) {
	key := c.Param("key")
	db := obj.GetDB(c, false)

	pkColName := db.NamingStrategy.ColumnName(obj.tableName, obj.PrimaryKeyName)

	var val T
	result := db.Where(pkColName, key).Delete(val)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, true)
}

func handleQueryObject[T any](c *gin.Context, obj *WebObject[T], prepareQuery PrepareQuery) {
	form, err := prepareQuery(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := obj.GetDB(c, false)
	namer := db.NamingStrategy

	// Use struct{} makes map like set.
	var filterFields = make(map[string]struct{})
	for _, k := range obj.Filters {
		filterFields[k] = struct{}{}
	}

	if len(filterFields) > 0 {
		var stripFilters []Filter
		for i := 0; i < len(form.Filters); i++ {
			f := form.Filters[i]
			// Struct must has this field.
			n, ok := obj.jsonToFields[f.Name]
			if !ok {
				continue
			}
			f.Name = n // replace to struct filed name
			if _, ok := filterFields[f.Name]; !ok {
				continue
			}
			f.Name = namer.ColumnName(obj.tableName, f.Name)
			stripFilters = append(stripFilters, f)
		}
		form.Filters = stripFilters
	} else {
		form.Filters = []Filter{}
	}

	var orderFields = make(map[string]struct{})
	for _, k := range obj.Orders {
		orderFields[k] = struct{}{}
	}

	if len(orderFields) > 0 {
		var stripOrders []Order
		for i := 0; i < len(form.Orders); i++ {
			o := form.Orders[i]
			n, ok := obj.jsonToFields[o.Name]
			if !ok {
				continue
			}
			o.Name = n
			if _, ok := orderFields[o.Name]; !ok {
				continue
			}
			o.Name = namer.ColumnName(obj.tableName, o.Name)
			stripOrders = append(stripOrders, o)
		}
		form.Orders = stripOrders
	} else {
		form.Orders = []Order{}
	}

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

// QueryObjects excute query and return data.
func QueryObjects[T any](db *gorm.DB, obj *WebObject[T], form *QueryForm) (r QueryResult[T], err error) {
	// the real name of the db table
	tblName := db.NamingStrategy.TableName(obj.tableName)

	for _, v := range form.Filters {
		q := v.GetQuery()
		if q != "" {
			db = db.Where(fmt.Sprintf("%s.%s", tblName, q), v.GetValue())
		}
	}

	for _, v := range form.Orders {
		q := v.GetQuery()
		if q != "" {
			db = db.Order(fmt.Sprintf("%s.%s", tblName, q))
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

	pos, limit := form.Pos, form.Limit
	if pos < 0 {
		pos = 0
	}
	if limit < 0 || limit > DefaultQueryLimit {
		limit = DefaultQueryLimit
	}

	r.Limit = limit
	r.Pos = form.Pos
	r.Keyword = form.Keyword

	var c int64
	var model T
	result := db.Model(model).Count(&c)
	if result.Error != nil {
		return r, result.Error
	}
	r.TotalCount = int(c)
	if c <= 0 {
		return r, nil
	}

	var items []T = make([]T, 0)
	db = db.Offset(form.Pos).Limit(limit)
	result = db.Find(&items)
	if result.Error != nil {
		return r, result.Error
	}
	r.Items = items
	r.Pos += int(result.RowsAffected)
	return r, nil
}

// DefaultPrepareQuery return default QueryForm.
func DefaultPrepareQuery(c *gin.Context) (*QueryForm, error) {
	var form QueryForm
	if c.Request.ContentLength > 0 {
		if err := c.BindJSON(&form); err != nil {
			return nil, err
		}
	}
	return &form, nil
}

/*
Check Go type corresponds to JSON type.
- float64, for JSON numbers
- string, for JSON strings
- []any, for JSON arrays
- map[string]any, for JSON objects
- nil, for JSON null
*/
func checkType(goKind, jsonKind reflect.Kind) bool {
	fmt.Println(goKind, jsonKind)
	switch goKind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return jsonKind == reflect.Float64
	default:
		return goKind == jsonKind
	}
}

// parseBool convert v to bool type. Unresolved is false.
// func parseBool(v any) (res bool, err error) {
// 	if val, ok := v.(bool); ok {
// 		return val, nil
// 	}
// 	if val, ok := v.(string); ok {
// 		return strconv.ParseBool(val)
// 	}
// 	return false, errors.New("parse bool type error")
// }

// ConverKey convert the kind of v to dst.
// func ConvertKey(dst reflect.Type, v any) any {
// 	if v == nil {
// 		return nil
// 	}
// 	src := reflect.TypeOf(v)
// 	if src.Kind() == dst.Kind() {
// 		return v
// 	}

// 	if src.Kind() == reflect.String {
// 		switch dst.Kind() {
// 		case reflect.Int:
// 			x, _ := strconv.ParseInt(v.(string), 10, 64)
// 			return int(x)
// 		case reflect.Int64:
// 			x, _ := strconv.ParseInt(v.(string), 10, 64)
// 			return x
// 		case reflect.Uint:
// 			x, _ := strconv.ParseUint(v.(string), 10, 64)
// 			return uint(x)
// 		case reflect.Uint64:
// 			x, _ := strconv.ParseUint(v.(string), 10, 64)
// 			return x
// 		case reflect.Bool:
// 			x, _ := strconv.ParseBool(v.(string))
// 			return x
// 		case reflect.Float32, reflect.Float64:
// 			x, _ := strconv.ParseFloat(v.(string), 64)
// 			return x
// 		}
// 	}
// 	return fmt.Sprintf("%v", v)
// }
