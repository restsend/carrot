package carrot

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminSettings struct {
	AdminTempalteDir string // default: "/admin/"
	assets           *StaticAssets
}

func (settings *AdminSettings) hintPage(objpath, name string) string {
	p := path.Join(settings.AdminTempalteDir, objpath, name)
	if settings.assets.Exists(p) {
		return p
	}
	return path.Join(settings.AdminTempalteDir, name)
}

// Access control
type AdminAccessCheck func(c *gin.Context, obj *AdminObject) error
type AdminAttribute struct {
}
type AdminField struct {
	Name     string
	Type     string
	Tag      string
	Attr     AdminAttribute
	Primary  bool
	elemType reflect.Type `json:"-"`
}

type AdminObject struct {
	Model          any                       `json:"-"`
	Group          string                    // Group name
	Name           string                    // Name of the object
	Path           string                    // Path prefix
	Shows          []string                  // Show fields
	Editables      []string                  // Editable fields
	Filterables    []string                  // Filterable fields
	Orderables     []string                  // Orderable fields
	Searchables    []string                  // Searchable fields
	ListPage       string                    // path to list page
	EditPage       string                    // path to edit/create page
	Pages          map[string]string         // path to custom pages
	Attributes     map[string]AdminAttribute // Field's extra attributes
	PrimaryKeyName string                    // Primary key name
	Fields         []AdminField

	AccessCheck AdminAccessCheck `json:"-"` // Access control function
	GetDB       GetDB            `json:"-"`
	OnCreate    CreateFunc       `json:"-"`
	OnUpdate    UpdateFunc       `json:"-"`
	OnDelete    DeleteFunc       `json:"-"`
	OnRender    RenderFunc       `json:"-"`
	tableName   string           `json:"-"`
	modelElem   reflect.Type     `json:"-"`
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
			}
			return
		}

		if !user.IsStaff && !user.IsSuperUser {
			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
	})

	if settings == nil {
		settings = &AdminSettings{
			AdminTempalteDir: "/admin/",
		}
	}

	settings.assets = as

	handledObjs := make([]AdminObject, 0)
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

		err = obj.RegisterAdmin(r, settings)
		if err != nil {
			Warning("RegisterAdmin fail, ignore", obj.Group, obj.Name, "err:", err)
			continue
		}
		handledObjs = append(handledObjs, *obj)
	}

	r.GET("/", func(ctx *gin.Context) {
		handleAdminIndex(ctx, settings, handledObjs)
	})
}

func handleAdminIndex(c *gin.Context, settings *AdminSettings, objs []AdminObject) {
	ctx := GetRenderPageContext(c)
	ctx["objects"] = objs
	ctx["user"] = CurrentUser(c)
	ctx["settings"] = settings

	htmlpage := path.Join(settings.AdminTempalteDir, "index.html")
	c.HTML(http.StatusOK, htmlpage, ctx)
}

// RegisterAdmin registers admin routes
//
//   - GET /admin/{objectslug}/ -> Get object page, lookup order:  {objectslug}/list.html, list.html
//   - GET /admin/{objectslug}/{pk} -> Get one page, lookup order:  {objectslug}/edit.html, edit.html
//   - POST /admin/{objectslug}/{pk} -> Get one object
//   - POST /admin/{objectslug} -> Get objects
//   - PUT /admin/{objectslug} -> Create One
//   - PATCH /admin/{objectslug}/{pk} -> Update One
//   - DELETE /admin/{objectslug}/{pk} -> Delete One
//   - POST /admin/_/action/{objectslug} -> Action
//   - POST /admin/_/render/{objectslug}/*filepath -> render html with object
func (obj *AdminObject) RegisterAdmin(r *gin.RouterGroup, settings *AdminSettings) (err error) {
	r.GET(obj.Path, func(c *gin.Context) {
		obj.handleGetListPage(c, settings)
	})

	r.GET(path.Join(obj.Path, ":pk"), func(c *gin.Context) {
		obj.handleGetEditPage(c, settings)
	})

	r.POST(path.Join(obj.Path, ":pk"), func(c *gin.Context) {
		obj.handleGetOne(c, settings)
	})

	r.POST(obj.Path, func(c *gin.Context) {
		obj.handleQuery(c, settings)
	})

	r.PUT(obj.Path, func(c *gin.Context) {
		obj.handleCreate(c, settings)
	})

	r.PATCH(path.Join(obj.Path, ":pk"), func(c *gin.Context) {
		obj.handleUpdate(c, settings)
	})

	r.DELETE(path.Join(obj.Path, ":pk"), func(c *gin.Context) {
		obj.handleDelete(c, settings)
	})

	r.POST(path.Join("_", "action", obj.Path), func(c *gin.Context) {
		obj.handleAction(c, settings)
	})

	r.POST(path.Join("_", "render", obj.Path, "*filepath"), func(c *gin.Context) {
		obj.handleRenderPage(c, settings)
	})

	return
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

		gormTag := f.Tag.Get("gorm")
		if gormTag == "" || gormTag == "-" {
			continue
		}
		field := AdminField{
			Name:     f.Name,
			Tag:      gormTag,
			Type:     f.Type.Name(),
			elemType: f.Type,
		}

		if strings.Contains(gormTag, "primarykey") ||
			strings.Contains(gormTag, "primaryKey") {
			if obj.PrimaryKeyName != "" {
				obj.PrimaryKeyName = f.Name
				field.Primary = true
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
	htmlpage := settings.hintPage(obj.Path, "list.html")
	ctx := GetRenderPageContext(c)
	ctx["user"] = CurrentUser(c)
	ctx["settings"] = settings
	ctx["object"] = obj
	c.HTML(http.StatusOK, htmlpage, ctx)
}

func (obj *AdminObject) handleGetEditPage(c *gin.Context, settings *AdminSettings) {
	htmlpage := settings.hintPage(obj.Path, "edit.html")
	ctx := GetRenderPageContext(c)
	ctx["user"] = CurrentUser(c)
	ctx["settings"] = settings
	ctx["object"] = obj
	ctx["pk"] = c.Param("pk")
	c.HTML(http.StatusOK, htmlpage, ctx)
}

func (obj *AdminObject) handleGetOne(c *gin.Context, settings *AdminSettings) {
	db := getDbConnection(c, obj.GetDB, false)
	pkColName := obj.GetColName(db, obj.PrimaryKeyName)

	modelObj := reflect.New(obj.modelElem).Interface()
	result := obj.GetDB(c, false).Where(pkColName, c.Param("pk")).First(modelObj)

	if result.Error != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": result.Error.Error(),
		})
		return
	}

	if obj.OnRender != nil {
		if err := obj.OnRender(c, modelObj); err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

// Query many objects with filter/limit/offset/order/search
func (obj *AdminObject) handleQuery(c *gin.Context, settings *AdminSettings) {
	db := getDbConnection(c, obj.GetDB, false)
	var form QueryForm
	if err := c.ShouldBind(&form); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// the real name of the db table
	tblName := db.NamingStrategy.TableName(obj.tableName)
	r, err := QueryObjectsEx(db, tblName, obj.modelElem, &form)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	if obj.OnRender != nil {
		vals := reflect.ValueOf(r.Items)
		if vals.Kind() == reflect.Slice {
			for i := 0; i < vals.Len(); i++ {
				v := vals.Index(i).Addr().Interface()
				if err := obj.OnRender(c, v); err != nil {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				vals.Index(i).Set(reflect.ValueOf(v).Elem())
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
	key := c.Param("key")

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
	key := c.Param("key")
	db := getDbConnection(c, obj.GetDB, false)

	pkColName := db.NamingStrategy.ColumnName(obj.tableName, obj.PrimaryKeyName)
	val := reflect.New(obj.modelElem).Interface()

	r := db.First(val, pkColName, key)

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

	r = db.Delete(&val)
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
	htmlpage := settings.hintPage(obj.Path, c.Param("filepath"))
	ctx := GetRenderPageContext(c)
	ctx["user"] = CurrentUser(c)
	ctx["settings"] = settings
	ctx["object"] = obj
	ctx["refer"] = c.Query("refer")

	c.HTML(http.StatusOK, htmlpage, ctx)
}
