package carrot

import (
	"database/sql"
	"encoding/json"
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

type Handle uint8

const (
	SINGLE_QUERY Handle = iota
	CREATE
	DELETE
	EDIT
	QUERY
)

type GetDB func(ctx *gin.Context, isCreate bool) *gorm.DB
type PrepareModel func(ctx *gin.Context, vptr any)
type PrepareQuery func(ctx *gin.Context, obj *WebObject) (*gorm.DB, *QueryForm, error)

type QueryView struct {
	Name    string
	Prepare PrepareQuery
}

type WebObject struct {
	Model     any
	Group     string
	Name      string
	Editables []string
	Filters   []string
	Orders    []string
	Searchs   []string
	GetDB     GetDB
	Init      PrepareModel // How to create a new object
	Views     []QueryView

	// Specify the to register to the route
	// (SINGLE_QUERY, CREATE, DELETE, EDIT, QUERY)
	// default register all handlers.
	Handlers []Handle

	PrimaryKeyName     string
	PrimaryKeyType     reflect.Type
	PrimaryKeyJsonName string
	tableName          string
	modelElem          reflect.Type

	// Map json tag to struct field name. such as:
	// UUID `json:"id"` => {"id" : "UUID"}
	jsonToFields map[string]string
}

type Filter struct {
	Name        string `json:"name"`
	Op          string `json:"op"`
	Value       string `json:"value"`
	targetValue any    `json:"-"`
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
	TotalCount int    `json:"total,omitempty"`
	Pos        int    `json:"pos,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Keyword    string `json:"keyword,omitempty"`
	Items      any    `json:"items,omitempty"`
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
	if f.targetValue == nil && f.Value != "" {
		return f.Value
	}
	if f.Op != FilterOpIn && f.Op != FilterOpNotIn {
		return f.targetValue
	}
	var arrValues []interface{}
	err := json.Unmarshal([]byte(f.Value), &arrValues)
	if err == nil {
		return arrValues
	}
	return f.targetValue
}

// GetQuery return the combined order SQL statement.
// such as "id DESC".
func (f *Order) GetQuery() string {
	if f.Op == OrderOpDesc {
		return f.Name + " DESC"
	}
	return f.Name
}

// ConverKey convert the kind of v to dst.
func ConvertKey(dst reflect.Type, v any) any {
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

// QueryObjects excute query and return data.
func QueryObjects(db *gorm.DB, obj *WebObject, form *QueryForm) (r QueryResult, err error) {
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

	limit := DefaultQueryLimit
	if form.Limit > 0 && form.Limit < DefaultQueryLimit {
		limit = form.Limit
	}

	r.Limit = limit
	r.Pos = form.Pos
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
	key := ConvertKey(obj.PrimaryKeyType, c.Param("key"))
	val := reflect.New(obj.modelElem).Interface()

	db := obj.GetDB(c, false)
	// the real name of the primaryKey column
	pkColName := db.NamingStrategy.ColumnName(obj.tableName, obj.PrimaryKeyName)
	result := db.Where(pkColName, key).Take(&val)
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
	key := ConvertKey(obj.PrimaryKeyType, c.Param("key"))

	var inputVals map[string]any
	err := c.BindJSON(&inputVals)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rt := obj.modelElem
	types := make(map[string]reflect.Type, 0)
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		types[f.Name] = f.Type
	}

	db := obj.GetDB(c, false)

	var vals map[string]any = map[string]any{}
	pkColName := db.NamingStrategy.ColumnName(obj.tableName, obj.PrimaryKeyName)
	delete(inputVals, obj.PrimaryKeyJsonName) // remove primaryKey

	for k, v := range inputVals {
		fname, ok := obj.jsonToFields[k]
		if !ok {
			continue
		}
		if rt, ok := types[fname]; ok {
			// Handle Illegal Values
			if rt.Kind() == reflect.Bool {
				v = parseBool(v)
			}
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

	if len(vals) <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "not changed"})
		return
	}

	model := reflect.New(obj.modelElem).Interface()
	result := db.Model(model).Where(pkColName, key).UpdateColumns(vals)

	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, true)
}

func handleDeleteObject(c *gin.Context, obj *WebObject) {
	key := ConvertKey(obj.PrimaryKeyType, c.Param("key"))
	val := reflect.New(obj.modelElem).Interface()

	db := obj.GetDB(c, false)
	pkColName := db.NamingStrategy.ColumnName(obj.tableName, obj.PrimaryKeyName)
	result := db.Where(pkColName, key).Delete(val)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, true)
}

// DefaultPrepareQuery return default QueryForm.
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

	// Use struct{} makes map like set.
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

func (obj *WebObject) RegisterObject(r gin.IRoutes) error {
	if err := obj.Build(); err != nil {
		return err
	}

	p := filepath.Join(obj.Group, obj.Name)

	handleMap := map[Handle]func(){
		SINGLE_QUERY: func() {
			r.GET(filepath.Join(p, ":key"), func(c *gin.Context) {
				handleGetObject(c, obj)
			})
		},
		CREATE: func() {
			r.PUT(p, func(c *gin.Context) {
				handleCreateObject(c, obj)
			})
		},
		EDIT: func() {
			r.PATCH(filepath.Join(p, ":key"), func(c *gin.Context) {
				handleEditObject(c, obj)
			})
		},
		DELETE: func() {
			r.DELETE(filepath.Join(p, ":key"), func(c *gin.Context) {
				handleDeleteObject(c, obj)
			})
		},
		QUERY: func() {
			r.POST(filepath.Join(p, "query"), func(c *gin.Context) {
				HandleQueryObject(c, obj, DefaultPrepareQuery)
			})
		},
	}

	// Register all by default.
	if len(obj.Handlers) == 0 {
		for _, f := range handleMap {
			f()
		}
	}

	for _, h := range obj.Handlers {
		if f, ok := handleMap[h]; ok {
			f()
			delete(handleMap, h) // Prevent duplicate registration.
		}
	}

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
	return nil
}

func RegisterObjects(r gin.IRoutes, objs []WebObject) {
	for idx := range objs {
		obj := &objs[idx]
		err := obj.RegisterObject(r)
		if err != nil {
			log.Printf("RegisterObject [%s] fail %v\n", obj.Name, err)
		}
	}
}

// parseFields parse the following properties according to struct tag:
// - jsonToFields, primaryKeyName, primaryKeyType, primaryKeyJsonName
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

		if !strings.Contains(gormTag, "primarykey") &&
			!strings.Contains(gormTag, "primaryKey") {
			continue
		}

		if jsonTag == "" || jsonTag == "-" {
			obj.PrimaryKeyJsonName = f.Name
		} else {
			obj.PrimaryKeyJsonName = jsonTag
		}
		obj.PrimaryKeyName = f.Name
		obj.PrimaryKeyType = f.Type
	}
}

// Build fill the properties of obj.
func (obj *WebObject) Build() error {
	obj.modelElem = reflect.TypeOf(obj.Model)
	if obj.modelElem.Kind() == reflect.Ptr {
		obj.modelElem = obj.modelElem.Elem()
	}

	obj.tableName = obj.modelElem.Name()
	if obj.Name == "" {
		obj.Name = strings.ToLower(obj.tableName)
	}

	obj.jsonToFields = make(map[string]string)

	obj.parseFields(obj.modelElem)
	if obj.PrimaryKeyName == "" {
		return fmt.Errorf("%s not primaryKey", obj.Name)
	}

	if obj.GetDB == nil {
		obj.GetDB = func(ctx *gin.Context, isCreate bool) *gorm.DB {
			return ctx.MustGet(DbField).(*gorm.DB)
		}
	}

	return nil
}

// parseBool convert v to bool type. Unresolved is false.
func parseBool(v any) (res bool) {
	if val, ok := v.(bool); ok {
		res = val
	} else if val, ok := v.(string); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			res = b
		} else {
			res = false
		}
	} else {
		res = false
	}
	return res
}
