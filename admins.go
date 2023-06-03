package carrot

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/inflection"
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
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Tag       string         `json:"tag,omitempty"`
	Attr      AdminAttribute `json:"attr"`
	Primary   bool           `json:"primary,omitempty"`
	IsAutoID  bool           `json:"isAutoId,omitempty"`
	elemType  reflect.Type   `json:"-"`
	fieldName string         `json:"-"`
}

type AdminObject struct {
	Model       any                       `json:"-"`
	Group       string                    `json:"group"`       // Group name
	Name        string                    `json:"name"`        // Name of the object
	Path        string                    `json:"path"`        // Path prefix
	Shows       []string                  `json:"shows"`       // Show fields
	Editables   []string                  `json:"editables"`   // Editable fields
	Filterables []string                  `json:"filterables"` // Filterable fields
	Orderables  []string                  `json:"orderables"`  // Orderable fields
	Searchables []string                  `json:"searchables"` // Searchable fields
	Attributes  map[string]AdminAttribute `json:"attributes"`  // Field's extra attributes
	PrimaryKey  string                    `json:"primaryKey"`  // Primary key name
	PluralName  string                    `json:"pluralName"`
	Fields      []AdminField              `json:"fields"`

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
	return []AdminObject{
		{
			Model:       &User{},
			Group:       "Sys",
			Name:        "User",
			Shows:       []string{"String", "Email", "Username", "FirstName", "ListName", "IsStaff", "IsSuperUser", "Enabled", "Actived", "Source", "Locale", "Timezone", "FirstName", "ListName"},
			Editables:   []string{"Email", "Password", "Username", "FirstName", "ListName", "IsStaff", "IsSuperUser", "Enabled", "Actived", "Source", "Locale", "Timezone"},
			Filterables: []string{"CreatedAt", "UpdatedAt", "Username", "IsStaff", "IsSuperUser", "Enabled", "Actived"},
			Orderables:  []string{"CreatedAt", "UpdatedAt", "Enabled", "Actived"},
			Searchables: []string{"Username", "Email", "FirstName", "ListName"},
		},
		{
			Model:       &Config{},
			Group:       "Sys",
			Name:        "Config",
			Shows:       []string{"Key", "Value", "Desc"},
			Editables:   []string{"Key", "Value", "Desc"},
			Orderables:  []string{"Key"},
			Searchables: []string{"Key", "Value", "Desc"},
			AccessCheck: func(c *gin.Context, obj *AdminObject) error {
				if !CurrentUser(c).IsSuperUser {
					return fmt.Errorf("only superuser can access")
				}
				return nil
			},
		},
	}
}

// RegisterAdmins registers admin routes
func RegisterAdmins(r *gin.RouterGroup, db *gorm.DB, objs []AdminObject) {
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

	r.GET("/admin.json", func(ctx *gin.Context) {
		handleAdminIndex(ctx, handledObjects)
	})
}

func handleAdminIndex(c *gin.Context, objects []*AdminObject) {
	c.JSON(http.StatusOK, gin.H{
		"objects": objects,
		"user":    CurrentUser(c),
		"site":    GetRenderPageContext(c),
	})
}

// RegisterAdmin registers admin routes
//
//   - GET /admin/{objectslug}/{:pk} -> Get object json
//   - POST /admin/{objectslug} -> Query objects
//   - PUT /admin/{objectslug} -> Create One
//   - PATCH /admin/{objectslug}/{pk} -> Update One
//   - DELETE /admin/{objectslug}/{pk} -> Delete One
//   - POST /admin/{objectslug}/_/{action}/:name -> Action
//   - POST /admin/{objectslug}/_/{render}/*filepath -> render html with object
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

	r.GET(":pk", obj.handleGetOne)
	r.POST("/", obj.handleQuery)
	r.PUT("/", obj.handleCreate)
	r.PATCH(":pk", obj.handleUpdate)
	r.DELETE(":pk", obj.handleDelete)
	r.POST("_/action/:name", obj.handleAction)
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

	err := obj.parseFields(db, rt)
	if err != nil {
		return err
	}
	if obj.PrimaryKey == "" {
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
			Type:      f.Type.Name(),
			elemType:  f.Type,
			fieldName: f.Name,
		}

		if obj.PrimaryKey == "" && strings.Contains(gormTag, "primarykey") {
			field.Primary = true
			obj.PrimaryKey = field.Name
			if strings.Contains(field.Type, "int") {
				field.IsAutoID = true
			}
		}
		obj.Fields = append(obj.Fields, field)
	}
	return nil
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

func (obj *AdminObject) handleGetOne(c *gin.Context) {
	db := getDbConnection(c, obj.GetDB, false)
	modelObj := reflect.New(obj.modelElem).Interface()
	result := db.Where(obj.PrimaryKey, c.Param("pk")).First(modelObj)

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

	for _, v := range form.Orders {
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
	r.Pos += int(result.RowsAffected)
	return r, nil
}

// Query many objects with filter/limit/offset/order/search
func (obj *AdminObject) handleQuery(c *gin.Context) {
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
	val := reflect.New(obj.modelElem).Interface()

	if err := c.BindJSON(&val); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if obj.BeforeCreate != nil {
		if err := obj.BeforeCreate(c, val); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	result := getDbConnection(c, obj.GetDB, true).Create(val)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, val)
}

func (obj *AdminObject) handleUpdate(c *gin.Context) {
	key := c.Param("pk")

	var inputVals map[string]any
	if err := c.BindJSON(&inputVals); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := getDbConnection(c, obj.GetDB, false)

	var vals map[string]any = map[string]any{}

	// can't edit primaryKey
	delete(inputVals, obj.PrimaryKey)
	fields := map[string]AdminField{}
	for _, v := range obj.Fields {
		fields[v.Name] = v
	}

	for k, v := range inputVals {
		if v == nil {
			continue
		}
		// Check the kind to be edited.
		f, ok := fields[k]
		if !ok {
			continue
		}

		if !checkType(f.elemType.Kind(), reflect.TypeOf(v).Kind()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s type not match", k)})
			return
		}

		vals[f.Name] = v
	}
	// if no editable fields, then all fields are editable
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

	if obj.BeforeUpdate != nil {
		val := reflect.New(obj.modelElem).Interface()
		if err := db.First(val, obj.PrimaryKey, key).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err := obj.BeforeUpdate(c, val, inputVals); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	model := reflect.New(obj.modelElem).Interface()
	result := db.Model(model).Where(obj.PrimaryKey, key).Updates(vals)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, true)
}

func (obj *AdminObject) handleDelete(c *gin.Context) {
	key := c.Param("pk")
	db := getDbConnection(c, obj.GetDB, false)

	//pkColName := db.NamingStrategy.ColumnName(obj.tableName, obj.PrimaryKeyName)
	val := reflect.New(obj.modelElem).Interface()

	r := db.Take(val, key)

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

	r = db.Delete(val)
	if r.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": r.Error.Error()})
		return
	}
	c.JSON(http.StatusOK, true)
}

func (obj *AdminObject) handleAction(c *gin.Context) {
	c.AbortWithStatus(http.StatusNotImplemented)
}
