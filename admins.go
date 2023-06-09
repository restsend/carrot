package carrot

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/inflection"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
)

type AdminQueryResult struct {
	TotalCount int              `json:"total,omitempty"`
	Pos        int              `json:"pos,omitempty"`
	Limit      int              `json:"limit,omitempty"`
	Keyword    string           `json:"keyword,omitempty"`
	Items      []map[string]any `json:"items"`
}

// Access control
type AdminAccessCheck func(c *gin.Context, obj *AdminObject) error
type AdminAttribute struct {
}
type AdminField struct {
	Label     string         `json:"label"` // Label of the object
	Required  bool           `json:"required,omitempty"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Tag       string         `json:"tag,omitempty"`
	Attr      AdminAttribute `json:"attr"`
	CanNull   bool           `json:"canNull,omitempty"`
	IsArray   bool           `json:"isArray,omitempty"`
	Primary   bool           `json:"primary,omitempty"`
	IsAutoID  bool           `json:"isAutoId,omitempty"`
	IsPtr     bool           `json:"isPtr,omitempty"`
	elemType  reflect.Type   `json:"-"`
	fieldName string         `json:"-"`
}
type AdminScript struct {
	Src    string `json:"src"`
	Onload bool   `json:"onload,omitempty"`
}
type AdminObject struct {
	Model       any                       `json:"-"`
	Group       string                    `json:"group"`                 // Group name
	Name        string                    `json:"name"`                  // Name of the object
	Placeholder string                    `json:"placeholder,omitempty"` // Placeholder of the object
	Desc        string                    `json:"desc,omitempty"`        // Description
	Path        string                    `json:"path"`                  // Path prefix
	Shows       []string                  `json:"shows"`                 // Show fields
	Orders      []Order                   `json:"-"`                     // Default orders of the object
	Editables   []string                  `json:"editables"`             // Editable fields
	Filterables []string                  `json:"filterables"`           // Filterable fields
	Orderables  []string                  `json:"orderables"`            // Orderable fields, can override Orders
	Searchables []string                  `json:"searchables"`           // Searchable fields
	Requireds   []string                  `json:"requireds,omitempty"`   // Required fields
	Attributes  map[string]AdminAttribute `json:"attributes"`            // Field's extra attributes
	PrimaryKey  []string                  `json:"primaryKey"`            // Primary key name
	PluralName  string                    `json:"pluralName"`
	Fields      []AdminField              `json:"fields"`
	EditPage    string                    `json:"editpage,omitempty"`
	ListPage    string                    `json:"listpage,omitempty"`
	Scripts     []AdminScript             `json:"scripts,omitempty"`
	Styles      []string                  `json:"styles,omitempty"`
	Permissions map[string]bool           `json:"permissions,omitempty"`

	AccessCheck  AdminAccessCheck `json:"-"` // Access control function
	GetDB        GetDB            `json:"-"`
	BeforeCreate BeforeCreateFunc `json:"-"`
	BeforeRender BeforeRenderFunc `json:"-"`
	BeforeUpdate BeforeUpdateFunc `json:"-"`
	BeforeDelete BeforeDeleteFunc `json:"-"`
	tableName    string           `json:"-"`
	modelElem    reflect.Type     `json:"-"`
}

// Returns all admin objects
func GetCarrotAdminObjects() []AdminObject {

	superAccessCheck := func(c *gin.Context, obj *AdminObject) error {
		if !CurrentUser(c).IsSuperUser {
			return errors.New("only superuser can access")
		}
		return nil
	}

	return []AdminObject{
		{
			Model:       &User{},
			Group:       "Settings",
			Name:        "User",
			Desc:        "Builtin user management system",
			Shows:       []string{"ID", "Email", "Username", "FirstName", "ListName", "IsStaff", "IsSuperUser", "Enabled", "Actived", "Source", "Locale", "Timezone", "LastLogin", "LastLoginIP"},
			Editables:   []string{"Email", "Password", "Username", "FirstName", "ListName", "IsStaff", "IsSuperUser", "Enabled", "Actived", "Source", "Locale", "Timezone"},
			Filterables: []string{"CreatedAt", "UpdatedAt", "Username", "IsStaff", "IsSuperUser", "Enabled", "Actived"},
			Orderables:  []string{"CreatedAt", "UpdatedAt", "Enabled", "Actived"},
			Searchables: []string{"Username", "Email", "FirstName", "ListName"},
			Orders:      []Order{{"CreatedAt", "desc"}},
			AccessCheck: superAccessCheck,
		},
		{
			Model:       &Config{},
			Group:       "Settings",
			Name:        "Config",
			Desc:        "System config with database backend, You can change it in admin page, and it will take effect immediately without restarting the server", //
			Shows:       []string{"Key", "Value", "Desc"},
			Editables:   []string{"Key", "Value", "Desc"},
			Orderables:  []string{"Key"},
			Searchables: []string{"Key", "Value", "Desc"},
			Requireds:   []string{"Key", "Value"},
			AccessCheck: superAccessCheck,
		},
	}
}

// RegisterAdmins registers admin routes
func RegisterAdmins(r *gin.RouterGroup, db *gorm.DB, adminAssetsRoot string, objs []AdminObject) {
	r.Use(func(ctx *gin.Context) {
		user := CurrentUser(ctx)
		if user == nil {
			db := ctx.MustGet(DbField).(*gorm.DB)
			signUrl := GetValue(db, KEY_SITE_SIGNIN_URL)
			if signUrl == "" {
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "login required"})
			} else {
				ctx.Redirect(http.StatusFound, signUrl+"?next="+ctx.Request.URL.String())
				ctx.Abort()
			}
			return
		}

		if !user.IsStaff && !user.IsSuperUser {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	})

	handledObjects := make([]*AdminObject, 0)
	exists := make(map[string]bool)
	for idx := range objs {
		obj := &objs[idx]
		err := obj.Build(db)
		if err != nil {
			Warning("Build admin object fail, ignore", obj.Group, obj.Name, "err:", err)
			continue
		}

		if _, ok := exists[obj.Path]; ok {
			Warning("Ignore exist admin object", obj.Group, obj.Name)
			continue
		}

		objr := r.Group(obj.Path)
		obj.Path = path.Join(r.BasePath(), obj.Path)
		obj.RegisterAdmin(objr)
		handledObjects = append(handledObjects, obj)
	}

	r.POST("/admin.json", func(ctx *gin.Context) {
		handleAdminIndex(ctx, handledObjects)
	})
	r.StaticFS("/", NewCombindEmbedFS(adminAssetsRoot, "admin", embedAdminAssets))
}

func handleAdminIndex(c *gin.Context, objects []*AdminObject) {
	var viewObjects []AdminObject
	for _, obj := range objects {
		if obj.AccessCheck != nil {
			err := obj.AccessCheck(c, obj)
			if err != nil {
				continue
			}
		}
		db := getDbConnection(c, obj.GetDB, false)
		val := *obj
		val.BuildPermissions(db, CurrentUser(c))
		viewObjects = append(viewObjects, val)
	}

	c.JSON(http.StatusOK, gin.H{
		"objects": viewObjects,
		"user":    CurrentUser(c),
		"site":    GetRenderPageContext(c),
	})
}

func (obj *AdminObject) BuildPermissions(db *gorm.DB, user *User) {
	obj.Permissions = map[string]bool{}
	if user.IsSuperUser {
		obj.Permissions["can_create"] = true
		obj.Permissions["can_update"] = true
		obj.Permissions["can_delete"] = true
		obj.Permissions["can_action"] = true
		return
	}

	//TODO: build permissions with group settings
	obj.Permissions["can_create"] = true
	obj.Permissions["can_update"] = true
	obj.Permissions["can_delete"] = true
	obj.Permissions["can_action"] = true
}

// RegisterAdmin registers admin routes
//
//   - POST /admin/{objectslug} -> Query objects
//   - PUT /admin/{objectslug} -> Create One
//   - PATCH /admin/{objectslug}} -> Update One
//   - DELETE /admin/{objectslug} -> Delete One
//   - POST /admin/{objectslug}/:name -> Action
func (obj *AdminObject) RegisterAdmin(r gin.IRoutes) {
	r = r.Use(func(ctx *gin.Context) {
		if obj.AccessCheck != nil {
			err := obj.AccessCheck(ctx, obj)
			if err != nil {
				ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": err.Error()})
				return
			}
		}
		ctx.Next()
	})

	r.POST("/", obj.handleQueryOrGetOne)
	r.PUT("/", obj.handleCreate)
	r.PATCH("/", obj.handleUpdate)
	r.DELETE("/", obj.handleDelete)
	r.POST("/_/:name", obj.handleAction)
}

func (obj *AdminObject) asColNames(db *gorm.DB, fields []string) []string {
	for i := 0; i < len(fields); i++ {
		fields[i] = db.NamingStrategy.ColumnName(obj.tableName, fields[i])
	}
	return fields
}

// Build fill the properties of obj.
func (obj *AdminObject) Build(db *gorm.DB) error {
	if obj.Path == "" {
		obj.Path = strings.ToLower(obj.Name)
	}

	if obj.Path == "_" || obj.Path == "" {
		return fmt.Errorf("invalid path")
	}

	rt := reflect.TypeOf(obj.Model)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	obj.modelElem = rt
	obj.tableName = db.NamingStrategy.TableName(rt.Name())
	obj.PluralName = inflection.Plural(obj.Name)
	obj.Shows = obj.asColNames(db, obj.Shows)
	obj.Editables = obj.asColNames(db, obj.Editables)
	obj.Orderables = obj.asColNames(db, obj.Orderables)
	obj.Searchables = obj.asColNames(db, obj.Searchables)
	obj.Filterables = obj.asColNames(db, obj.Filterables)
	obj.Requireds = obj.asColNames(db, obj.Requireds)

	for idx := range obj.Orders {
		o := &obj.Orders[idx]
		o.Name = db.NamingStrategy.ColumnName(obj.tableName, o.Name)
	}

	err := obj.parseFields(db, rt)
	if err != nil {
		return err
	}
	if len(obj.PrimaryKey) <= 0 {
		return fmt.Errorf("%s not has primaryKey", obj.Name)
	}
	return nil
}

func (obj *AdminObject) parseFields(db *gorm.DB, rt reflect.Type) error {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)

		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			obj.parseFields(db, f.Type)
		}

		gormTag := strings.ToLower(f.Tag.Get("gorm"))
		if gormTag == "-" {
			continue
		}
		field := AdminField{
			Name:      db.NamingStrategy.ColumnName(obj.tableName, f.Name),
			Tag:       gormTag,
			elemType:  f.Type,
			fieldName: f.Name,
			Label:     f.Tag.Get("label"),
		}
		if field.elemType.Kind() == reflect.Ptr {
			field.elemType = field.elemType.Elem()
		}
		if field.Label == "" {
			field.Label = strings.ReplaceAll(field.Name, "_", " ")
		}

		field.Label = cases.Title(language.Und).String(field.Label)

		switch f.Type.Kind() {
		case reflect.Ptr:
			field.Type = f.Type.Elem().Name()
			field.CanNull = true
			field.IsPtr = true
		case reflect.Slice:
			field.Type = f.Type.Elem().Name()
			field.CanNull = true
			field.IsArray = true
		default:
			field.Type = f.Type.Name()
		}

		if field.Type == "NullTime" || field.Type == "Time" {
			field.Type = "datetime"
		}

		if strings.Contains(gormTag, "primarykey") || strings.Contains(gormTag, "uniqueindex") {
			field.Primary = true
			obj.PrimaryKey = append(obj.PrimaryKey, field.Name)
			if strings.Contains(field.Type, "int") {
				field.IsAutoID = true
			}
		}
		obj.Fields = append(obj.Fields, field)
	}
	return nil
}

func (f *AdminField) convertValue(source any) (any, error) {
	srcType := reflect.TypeOf(source)
	if srcType == f.elemType {
		return source, nil
	}
	var targetType reflect.Type = f.elemType
	var err error
	switch f.Type {
	case "int", "int8", "int16", "int32", "int64":
		v, err := strconv.ParseInt(fmt.Sprintf("%v", source), 10, 64)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(v).Convert(targetType).Interface(), nil
	case "uint", "uint8", "uint16", "uint32", "uint64":
		v, err := strconv.ParseUint(fmt.Sprintf("%v", source), 10, 64)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(v).Convert(targetType).Interface(), nil
	case "float32", "float64":
		v, err := strconv.ParseFloat(fmt.Sprintf("%v", source), 64)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(v).Convert(targetType).Interface(), nil
	case "bool":
		v, err := strconv.ParseBool(fmt.Sprintf("%v", source))
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(v).Interface(), nil
	case "string":
		return fmt.Sprintf("%v", source), nil
	case "datetime":
		if fmt.Sprintf("%v", source) == "" {
			return &time.Time{}, nil
		} else {
			tv := fmt.Sprintf("%v", source)
			for _, tf := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02", time.RFC1123} {
				t, err := time.Parse(tf, tv)
				if err == nil {
					return &t, nil
				}
			}
		}
		return nil, fmt.Errorf("invalid datetime format %v", source)
	default:
		var data []byte
		data, err = json.Marshal(source)
		if err != nil {
			return nil, err
		}
		value := reflect.New(targetType).Interface()
		err = json.Unmarshal(data, value)
		return value, err
	}
}

func (obj *AdminObject) UnmarshalFrom(keys, vals map[string]any) (any, error) {
	elemObj := reflect.New(obj.modelElem)
	if len(obj.Editables) > 0 {
		editables := make(map[string]bool)
		for _, v := range obj.Editables {
			editables[v] = true
		}
		for k := range vals {
			if _, ok := editables[k]; !ok {
				delete(vals, k)
			}
		}
	}

	for k, v := range keys {
		vals[k] = v
	}

	for _, field := range obj.Fields {
		val, ok := vals[field.Name]
		if !ok {
			continue
		}

		if val == nil {
			continue
		}
		fieldValue, err := field.convertValue(val)
		if err != nil {
			return nil, fmt.Errorf("invalid type: %s except: %s actual: %s error:%v", field.Name, field.Type, reflect.TypeOf(val).Name(), err)
		}

		target := elemObj.Elem().FieldByName(field.fieldName)
		targetValue := reflect.ValueOf(fieldValue)

		if target.Kind() == reflect.Ptr {
			ptrValue := reflect.New(reflect.PointerTo(field.elemType))
			ptrValue.Elem().Set(targetValue)
			targetValue = ptrValue.Elem()
		} else {
			if targetValue.Kind() == reflect.Ptr {
				targetValue = targetValue.Elem()
			}
		}

		target.Set(targetValue)
	}
	return elemObj.Interface(), nil
}

func (obj *AdminObject) MarshalOne(val interface{}) (map[string]any, error) {
	var result = make(map[string]any)
	rv := reflect.ValueOf(val)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	for _, field := range obj.Fields {
		var fieldval any
		if v := rv.MethodByName(field.fieldName); v.IsValid() {
			r := v.Call(nil)
			if len(r) > 0 {
				fieldval = r[0].Interface()
			}
		} else {
			v := rv.FieldByName(field.fieldName)
			if v.IsValid() {
				fieldval = v.Interface()
			}
		}
		result[field.Name] = fieldval
	}
	return result, nil
}

func (obj *AdminObject) getPrimaryValues(c *gin.Context) map[string]any {
	var result = make(map[string]any)
	for _, field := range obj.PrimaryKey {
		if v := c.Query(field); v != "" {
			result[field] = v
		}
	}
	return result
}

func (obj *AdminObject) handleGetOne(c *gin.Context) {
	db := getDbConnection(c, obj.GetDB, false)
	modelObj := reflect.New(obj.modelElem).Interface()
	keys := obj.getPrimaryValues(c)
	if len(keys) <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid primary key",
		})
		return
	}

	result := db.Where(keys).First(modelObj)

	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": result.Error.Error(),
		})
		return
	}

	if obj.BeforeRender != nil {
		err := obj.BeforeRender(c, modelObj)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	data, err := obj.MarshalOne(modelObj)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, data)
}

func (obj *AdminObject) QueryObjects(db *gorm.DB, form *QueryForm, ctx *gin.Context) (r AdminQueryResult, err error) {
	for _, v := range form.Filters {
		if q := v.GetQuery(); q != "" {
			if v.Op == FilterOpLike {
				kw := sql.Named("keyword", fmt.Sprintf(`%%%s%%`, v.Value))
				db = db.Where(fmt.Sprintf("`%s`.%s @keyword", obj.tableName, q), kw)
			} else {
				db = db.Where(fmt.Sprintf("`%s`.%s", obj.tableName, q), v.Value)
			}
		}
	}

	var orders []Order
	if len(form.Orders) > 0 {
		orders = form.Orders
	} else {
		orders = obj.Orders
	}

	for _, v := range orders {
		if q := v.GetQuery(); q != "" {
			db = db.Order(fmt.Sprintf("`%s`.%s", obj.tableName, q))
		}
	}

	if form.Keyword != "" && len(obj.Searchables) > 0 {
		var query []string
		for _, v := range obj.Searchables {
			query = append(query, fmt.Sprintf("`%s`.`%s` LIKE @keyword", obj.tableName, v))
		}
		searchKey := strings.Join(query, " OR ")
		db = db.Where(searchKey, sql.Named("keyword", "%"+form.Keyword+"%"))
	}

	r.Pos = form.Pos
	r.Limit = form.Limit
	r.Keyword = form.Keyword

	db = db.Table(obj.tableName)

	var c int64
	if err := db.Count(&c).Error; err != nil {
		return r, err
	}
	if c <= 0 {
		return r, nil
	}
	r.TotalCount = int(c)

	selected := []string{}
	for _, v := range obj.Fields {
		selected = append(selected, v.Name)
	}

	vals := reflect.New(reflect.SliceOf(obj.modelElem))
	result := db.Select(selected).Offset(form.Pos).Limit(form.Limit).Find(vals.Interface())
	if result.Error != nil {
		return r, result.Error
	}

	for i := 0; i < vals.Elem().Len(); i++ {
		modelObj := vals.Elem().Index(i).Interface()
		if obj.BeforeRender != nil {
			err := obj.BeforeRender(ctx, modelObj)
			if err != nil {
				return r, err
			}
		}
		item, err := obj.MarshalOne(modelObj)
		if err != nil {
			return r, err
		}
		r.Items = append(r.Items, item)
	}
	return r, nil
}

// Query many objects with filter/limit/offset/order/search
func (obj *AdminObject) handleQueryOrGetOne(c *gin.Context) {
	if c.Request.ContentLength <= 0 {
		obj.handleGetOne(c)
		return
	}

	db, form, err := DefaultPrepareQuery(getDbConnection(c, obj.GetDB, false), c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	r, err := obj.QueryObjects(db, form, c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, r)
}

func (obj *AdminObject) handleCreate(c *gin.Context) {
	keys := obj.getPrimaryValues(c)
	var vals map[string]any
	if err := c.BindJSON(&vals); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	elmObj, err := obj.UnmarshalFrom(keys, vals)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if obj.BeforeCreate != nil {
		if err := obj.BeforeCreate(c, elmObj, vals); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	result := getDbConnection(c, obj.GetDB, true).Create(elmObj)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, elmObj)
}

func (obj *AdminObject) handleUpdate(c *gin.Context) {
	keys := obj.getPrimaryValues(c)
	if len(keys) <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid primary key",
		})
		return
	}

	var inputVals map[string]any
	if err := c.BindJSON(&inputVals); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := getDbConnection(c, obj.GetDB, false)
	val, err := obj.UnmarshalFrom(keys, inputVals)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if obj.BeforeUpdate != nil {
		if err := db.Where(keys).First(val).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err := obj.BeforeUpdate(c, val, inputVals); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	result := db.Where(keys).Save(val)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, true)
}

func (obj *AdminObject) handleDelete(c *gin.Context) {
	keys := obj.getPrimaryValues(c)
	if len(keys) <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "invalid primary key",
		})
		return
	}
	db := getDbConnection(c, obj.GetDB, false)
	val := reflect.New(obj.modelElem).Interface()
	r := db.Where(keys).Take(val)

	// for gorm delete hook, need to load model first.
	if r.Error != nil {
		if errors.Is(r.Error, gorm.ErrRecordNotFound) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": r.Error.Error()})
		}
		return
	}

	if obj.BeforeDelete != nil {
		if err := obj.BeforeDelete(c, val); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	r = db.Where(keys).Delete(val)
	if r.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": r.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, true)
}

func (obj *AdminObject) handleAction(c *gin.Context) {
	c.AbortWithStatus(http.StatusNotImplemented)
}
