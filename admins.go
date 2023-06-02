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
type AdminSettings struct {
	Title          string        `json:"title"`
	TempalteRoot   string        `json:"-"`        // default: "/admin/"
	ListPage       string        `json:"-"`        // default: "list.html"
	EditPage       string        `json:"-"`        // default: "edit.html"
	PetiteVueURL   string        `json:"petite"`   // default: ""
	TailwindCSSURL string        `json:"tailwind"` // default: ""
	Prefix         string        `json:"prefix"`   // default: "/admin/"
	Objects        []AdminObject `json:"-"`
	assets         *StaticAssets `json:"-"`
}

func (settings *AdminSettings) hintPage(objpath, name string) string {
	p := path.Join(settings.TempalteRoot, objpath, name)
	if settings.assets != nil && settings.assets.TemplateExists(p) {
		return p
	}
	return path.Join(settings.TempalteRoot, name)
}

// Access control
type AdminAccessCheck func(c *gin.Context, obj *AdminObject) error
type AdminAttribute struct {
}
type AdminField struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Tag      string         `json:"tag"`
	Attr     AdminAttribute `json:"attr"`
	Primary  bool           `json:"primary"`
	IsAutoID bool           `json:"isAutoId"`
	elemType reflect.Type   `json:"-"`
}

type AdminObject struct {
	Model          any                       `json:"-"`
	Group          string                    `json:"group"`       // Group name
	Name           string                    `json:"name"`        // Name of the object
	Path           string                    `json:"path"`        // Path prefix
	Shows          []string                  `json:"shows"`       // Show fields
	Editables      []string                  `json:"editables"`   // Editable fields
	Filterables    []string                  `json:"filterables"` // Filterable fields
	Orderables     []string                  `json:"orderables"`  // Orderable fields
	Searchables    []string                  `json:"searchables"` // Searchable fields
	Attributes     map[string]AdminAttribute `json:"attributes"`  // Field's extra attributes
	PrimaryKeyName string                    `json:"primaryKey"`  // Primary key name
	Fields         []AdminField              `json:"fields"`

	AccessCheck AdminAccessCheck `json:"-"` // Access control function
	GetDB       GetDB            `json:"-"`
	OnCreate    CreateFunc       `json:"-"`
	OnUpdate    UpdateFunc       `json:"-"`
	OnDelete    DeleteFunc       `json:"-"`
	tableName   string           `json:"-"`
	modelElem   reflect.Type     `json:"-"`
	PluralName  string           `json:"pluralName"`
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
func RegisterAdmins(r *gin.RouterGroup, as *StaticAssets, objs []AdminObject, settings *AdminSettings) {
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

	if settings == nil {
		settings = &AdminSettings{}
	}

	if settings.Prefix == "" {
		settings.Prefix = "/admin/"
	}
	if settings.TempalteRoot == "" {
		settings.TempalteRoot = "/admin/"
	}
	if settings.Title == "" {
		settings.Title = "Carrot Admin"
	}
	settings.assets = as
	RegisterCarrotFilters()

	settings.Objects = make([]AdminObject, 0)
	exists := make(map[string]bool)
	for idx := range objs {
		obj := &objs[idx]
		err := obj.Build()
		if err != nil {
			Warning("Build admin object fail, ignore", obj.Group, obj.Name, "err:", err)
			continue
		}

		if _, ok := exists[obj.Path]; ok {
			Warning("Ignore exist admin object", obj.Group, obj.Name)
			continue
		}

		objr := r.Group(obj.Path)
		obj.RegisterAdmin(objr, settings)
		settings.Objects = append(settings.Objects, *obj)
	}

	r.GET("/", func(ctx *gin.Context) {
		handleAdminIndex(ctx, settings)
	})
}

func handleAdminIndex(c *gin.Context, settings *AdminSettings) {
	ctx := GetRenderPageContext(c)
	ctx["settings"] = settings
	ctx["user"] = CurrentUser(c)

	htmlpage := path.Join(settings.TempalteRoot, "index.html")
	c.HTML(http.StatusOK, htmlpage, ctx)
}

// RegisterAdmin registers admin routes
//
//   - GET /admin/{objectslug}/ -> Get object page, lookup order:  {objectslug}/list.html, list.html
//   - GET /admin/{objectslug}/{:pk} -> Get object json
//   - POST /admin/{objectslug} -> Get objects
//   - PUT /admin/{objectslug} -> Create One
//   - PATCH /admin/{objectslug}/{pk} -> Update One
//   - DELETE /admin/{objectslug}/{pk} -> Delete One
//   - POST /admin/{objectslug}/_/{action}/:name -> Action
//   - POST /admin/{objectslug}/_/{render}/*filepath -> render html with object
func (obj *AdminObject) RegisterAdmin(r gin.IRoutes, settings *AdminSettings) {
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

	r.GET("/", func(c *gin.Context) {
		obj.handleGetListPage(c, settings)
	})

	r.GET(":pk", func(c *gin.Context) {
		obj.handleGetOne(c, settings)
	})

	r.POST("/", func(c *gin.Context) {
		obj.handleQuery(c, settings)
	})

	r.PUT("/", func(c *gin.Context) {
		obj.handleCreate(c, settings)
	})

	r.PATCH(":pk", func(c *gin.Context) {
		obj.handleUpdate(c, settings)
	})

	r.DELETE(":pk", func(c *gin.Context) {
		obj.handleDelete(c, settings)
	})

	r.POST("_/action/:name", func(c *gin.Context) {
		obj.handleAction(c, settings)
	})

	r.POST("_/render/*filepath", func(c *gin.Context) {
		obj.handleRenderPage(c, settings)
	})
}

// Build fill the properties of obj.
func (obj *AdminObject) Build() error {
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
	obj.tableName = rt.Name()
	obj.PluralName = inflection.Plural(obj.Name)
	err := obj.parseFields(rt)
	if err != nil {
		return err
	}
	if obj.PrimaryKeyName == "" {
		return fmt.Errorf("%s not has primaryKey", obj.Name)
	}
	return nil
}

func (obj *AdminObject) GetColName(db *gorm.DB, field string) string {
	return db.NamingStrategy.ColumnName(obj.tableName, field)
}

func (obj *AdminObject) parseFields(rt reflect.Type) error {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)

		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			obj.parseFields(f.Type)
		}

		gormTag := strings.ToLower(f.Tag.Get("gorm"))
		if gormTag == "-" {
			continue
		}
		field := AdminField{
			Name:     f.Name,
			Tag:      gormTag,
			Type:     f.Type.Name(),
			elemType: f.Type,
		}

		if obj.PrimaryKeyName == "" && strings.Contains(gormTag, "primarykey") {
			obj.PrimaryKeyName = f.Name
			field.Primary = true
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
	if rv.IsNil() || rv.IsZero() {
		return result, nil
	}
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	for _, field := range obj.Fields {
		var fieldval any
		if v := rv.MethodByName(field.Name); v.IsValid() {
			//TODO: call method
			r := v.Call(nil)
			if len(r) > 0 {
				fieldval = r[0].Interface()
			}
		} else {
			v := rv.FieldByName(field.Name)
			if v.IsValid() {
				fieldval = v.Interface()
			}
		}
		result[field.Name] = fieldval
	}
	return result, nil
}

func (obj *AdminObject) handleGetListPage(c *gin.Context, settings *AdminSettings) {
	htmlpage := settings.ListPage
	if htmlpage == "" {
		htmlpage = "list.html"
	}
	htmlpage = settings.hintPage(obj.Path, htmlpage)
	ctx := GetRenderPageContext(c)
	ctx["user"] = CurrentUser(c)
	ctx["settings"] = settings
	ctx["current"] = obj

	c.HTML(http.StatusOK, htmlpage, ctx)
}

func (obj *AdminObject) handleGetOne(c *gin.Context, settings *AdminSettings) {
	db := getDbConnection(c, obj.GetDB, false)
	pkColName := obj.GetColName(db, obj.PrimaryKeyName)

	modelObj := reflect.New(obj.modelElem).Interface()
	result := db.Where(pkColName, c.Param("pk")).First(modelObj)

	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": result.Error.Error(),
		})
		return
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

func (obj *AdminObject) QueryObjects(db *gorm.DB, form *QueryForm) (r AdminQueryResult, err error) {
	tblName := db.NamingStrategy.TableName(obj.tableName)

	for _, v := range form.Filters {
		v.Name = fmt.Sprintf("`%s`", db.NamingStrategy.ColumnName(obj.tableName, v.Name))
		if q := v.GetQuery(); q != "" {
			value := v.Value
			if v.Op == FilterOpLike {
				value = fmt.Sprintf(`%%%s%%`, value)
			}
			db = db.Where(fmt.Sprintf("`%s`.%s", tblName, q), value)
		}
	}

	for _, v := range form.Orders {
		if q := v.GetQuery(); q != "" {
			db = db.Order(fmt.Sprintf("%s.%s", tblName, q))
		}
	}

	if form.Keyword != "" && len(obj.Searchables) > 0 {
		var query []string
		for _, v := range obj.Searchables {
			colName := db.NamingStrategy.ColumnName(obj.tableName, v)
			query = append(query, fmt.Sprintf("`%s`.`%s` LIKE @keyword", tblName, colName))
		}
		searchKey := strings.Join(query, " OR ")
		db = db.Where(searchKey, sql.Named("keyword", "%"+form.Keyword+"%"))
	}

	r.Pos = form.Pos
	r.Limit = form.Limit
	r.Keyword = form.Keyword

	db = db.Table(tblName)
	var c int64
	if err := db.Count(&c).Error; err != nil {
		return r, err
	}
	if c <= 0 {
		return r, nil
	}
	r.TotalCount = int(c)

	items := []map[string]any{}
	result := db.Offset(form.Pos).Limit(form.Limit).Find(&items)
	if result.Error != nil {
		return r, result.Error
	}
	r.Items = items
	r.Pos += int(result.RowsAffected)
	return r, nil
}

// Query many objects with filter/limit/offset/order/search
func (obj *AdminObject) handleQuery(c *gin.Context, settings *AdminSettings) {
	db, form, err := DefaultPrepareQuery(getDbConnection(c, obj.GetDB, false), c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// the real name of the db table
	mapping := map[string]string{}
	for _, v := range obj.Fields {
		mapping[db.NamingStrategy.ColumnName(obj.tableName, v.Name)] = v.Name
	}

	r, err := obj.QueryObjects(db, form)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	for _, item := range r.Items {
		for k, v := range item { // convert column name to field name
			if name, ok := mapping[k]; ok {
				item[name] = v
				delete(item, k)
			}
		}
	}
	c.JSON(http.StatusOK, r)
}

func (obj *AdminObject) handleCreate(c *gin.Context, settings *AdminSettings) {
	val := reflect.New(obj.modelElem).Interface()

	if err := c.BindJSON(&val); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if obj.OnCreate != nil {
		if err := obj.OnCreate(c, val); err != nil {
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

func (obj *AdminObject) handleUpdate(c *gin.Context, settings *AdminSettings) {
	key := c.Param("pk")

	var inputVals map[string]any
	if err := c.BindJSON(&inputVals); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := getDbConnection(c, obj.GetDB, false)

	var vals map[string]any = map[string]any{}

	// can't edit primaryKey
	delete(inputVals, obj.PrimaryKeyName)
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

	pkColName := db.NamingStrategy.ColumnName(obj.tableName, obj.PrimaryKeyName)

	if obj.OnUpdate != nil {
		val := reflect.New(obj.modelElem).Interface()
		if err := db.First(val, pkColName, key).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err := obj.OnUpdate(c, val, inputVals); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	model := reflect.New(obj.modelElem).Interface()
	result := db.Model(model).Where(pkColName, key).Updates(vals)
	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, true)
}

func (obj *AdminObject) handleDelete(c *gin.Context, settings *AdminSettings) {
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

	if obj.OnDelete != nil {
		if err := obj.OnDelete(c, val); err != nil {
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

func (obj *AdminObject) handleAction(c *gin.Context, settings *AdminSettings) {
	c.AbortWithStatus(http.StatusNotImplemented)
}

func (obj *AdminObject) handleRenderPage(c *gin.Context, settings *AdminSettings) {
	ext := path.Ext(c.Param("filepath"))
	if strings.ToLower(ext) != ".html" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if settings.assets == nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	htmlpage := path.Join(settings.TempalteRoot, obj.Path, c.Param("filepath"))
	if !settings.assets.TemplateExists(htmlpage) {
		htmlpage = path.Join(settings.TempalteRoot, c.Param("filepath"))
		if !settings.assets.TemplateExists(htmlpage) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}

	ctx := GetRenderPageContext(c)
	ctx["user"] = CurrentUser(c)
	ctx["settings"] = settings
	ctx["current"] = obj
	ctx["refer"] = c.Query("refer")

	c.HTML(http.StatusOK, htmlpage, ctx)
}
