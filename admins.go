package carrot

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/inflection"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const KEY_ADMIN_DASHBOARD = "ADMIN_DASHBOARD"

type AdminQueryResult struct {
	TotalCount int              `json:"total,omitempty"`
	Pos        int              `json:"pos,omitempty"`
	Limit      int              `json:"limit,omitempty"`
	Keyword    string           `json:"keyword,omitempty"`
	Items      []map[string]any `json:"items"`
	objects    []any            `json:"-"`
}

// Access control
type AdminAccessCheck func(c *gin.Context, obj *AdminObject) error
type AdminActionHandler func(db *gorm.DB, c *gin.Context, obj any) (any, error)
type AdminSelectOption struct {
	Label string `json:"label"`
	Value any    `json:"value"`
}

type AdminAttribute struct {
	Default any                 `json:"default,omitempty"`
	Choices []AdminSelectOption `json:"choices,omitempty"`
}
type AdminForeign struct {
	Path       string `json:"path"`
	Field      string `json:"field"`
	fieldName  string `json:"-"`
	foreignKey string `json:"-"`
}
type AdminValue struct {
	Value any    `json:"value"`
	Label string `json:"label,omitempty"`
}

type AdminField struct {
	Label     string          `json:"label"` // Label of the object
	Required  bool            `json:"required,omitempty"`
	Name      string          `json:"name"`
	Type      string          `json:"type"`
	Tag       string          `json:"tag,omitempty"`
	Attribute *AdminAttribute `json:"attribute,omitempty"`
	CanNull   bool            `json:"canNull,omitempty"`
	IsArray   bool            `json:"isArray,omitempty"`
	Primary   bool            `json:"primary,omitempty"`
	Foreign   *AdminForeign   `json:"foreign,omitempty"`
	IsAutoID  bool            `json:"isAutoId,omitempty"`
	IsPtr     bool            `json:"isPtr,omitempty"`
	elemType  reflect.Type    `json:"-"`
	fieldName string          `json:"-"`
}
type AdminScript struct {
	Src    string `json:"src"`
	Onload bool   `json:"onload,omitempty"`
}
type AdminAction struct {
	Path    string             `json:"path"`
	Name    string             `json:"name"`
	Label   string             `json:"label,omitempty"`
	Icon    string             `json:"icon,omitempty"`
	Class   string             `json:"class,omitempty"`
	Handler AdminActionHandler `json:"-"`
}

type AdminObject struct {
	Model       any             `json:"-"`
	Group       string          `json:"group"`                 // Group name
	Name        string          `json:"name"`                  // Name of the object
	Placeholder string          `json:"placeholder,omitempty"` // Placeholder of the object
	Desc        string          `json:"desc,omitempty"`        // Description
	Path        string          `json:"path"`                  // Path prefix
	Shows       []string        `json:"shows"`                 // Show fields
	Orders      []Order         `json:"-"`                     // Default orders of the object
	Editables   []string        `json:"editables"`             // Editable fields
	Filterables []string        `json:"filterables"`           // Filterable fields
	Orderables  []string        `json:"orderables"`            // Orderable fields, can override Orders
	Searchables []string        `json:"searchables"`           // Searchable fields
	Requireds   []string        `json:"requireds,omitempty"`   // Required fields
	PrimaryKey  []string        `json:"primaryKey"`            // Primary key name
	PluralName  string          `json:"pluralName"`
	Fields      []AdminField    `json:"fields"`
	EditPage    string          `json:"editpage,omitempty"`
	ListPage    string          `json:"listpage,omitempty"`
	Scripts     []AdminScript   `json:"scripts,omitempty"`
	Styles      []string        `json:"styles,omitempty"`
	Permissions map[string]bool `json:"permissions,omitempty"`
	Actions     []AdminAction   `json:"actions,omitempty"`

	Attributes   map[string]AdminAttribute `json:"-"` // Field's extra attributes
	AccessCheck  AdminAccessCheck          `json:"-"` // Access control function
	GetDB        GetDB                     `json:"-"`
	BeforeCreate BeforeCreateFunc          `json:"-"`
	BeforeRender BeforeRenderFunc          `json:"-"`
	BeforeUpdate BeforeUpdateFunc          `json:"-"`
	BeforeDelete BeforeDeleteFunc          `json:"-"`
	tableName    string                    `json:"-"`
	modelElem    reflect.Type              `json:"-"`
	ignores      map[string]bool           `json:"-"`
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
			Shows:       []string{"ID", "Email", "Username", "FirstName", "ListName", "IsStaff", "IsSuperUser", "Enabled", "Activated", "Profile", "Source", "Locale", "Timezone", "LastLogin", "LastLoginIP"},
			Editables:   []string{"Email", "Password", "Username", "FirstName", "ListName", "IsStaff", "IsSuperUser", "Enabled", "Activated", "Profile", "Source", "Locale", "Timezone"},
			Filterables: []string{"CreatedAt", "UpdatedAt", "Username", "IsStaff", "IsSuperUser", "Enabled", "Activated "},
			Orderables:  []string{"CreatedAt", "UpdatedAt", "Enabled", "Activated"},
			Searchables: []string{"Username", "Email", "FirstName", "ListName"},
			Orders:      []Order{{"UpdatedAt", OrderOpDesc}},
			AccessCheck: superAccessCheck,
			Actions: []AdminAction{
				{
					Path:  "toggle_enabled",
					Name:  "Toggle enabled",
					Label: "Toggle user enabled/disabled",
					Handler: func(db *gorm.DB, c *gin.Context, obj any) (any, error) {
						user := obj.(*User)
						err := UpdateUserFields(db, user, map[string]any{"Enabled": !user.Enabled})
						return user.Enabled, err
					},
				},
				{
					Path:  "toggle_staff",
					Name:  "Toggle staff",
					Label: "Toggle user is staff or not",
					Handler: func(db *gorm.DB, c *gin.Context, obj any) (any, error) {
						user := obj.(*User)
						err := UpdateUserFields(db, user, map[string]any{"IsStaff": !user.IsStaff})
						return user.IsStaff, err
					},
				},
			},
		},
		{
			Model:       &Group{},
			Group:       "Settings",
			Name:        "Group",
			Desc:        "A group describes a group of users. One user can be part of many groups and one group can have many users", //
			Shows:       []string{"ID", "Name", "Extra", "UpdatedAt", "CreatedAt"},
			Editables:   []string{"ID", "Name", "Extra", "UpdatedAt"},
			Orderables:  []string{"UpdatedAt"},
			Searchables: []string{"Name"},
			Requireds:   []string{"Name"},
			AccessCheck: superAccessCheck,
		},
		{
			Model:       &GroupMember{},
			Group:       "Settings",
			Name:        "GroupMember",
			Desc:        "Group members", //
			Shows:       []string{"ID", "User", "Group", "Role", "CreatedAt"},
			Editables:   []string{"ID", "User", "Group", "Role"},
			Orderables:  []string{"CreatedAt"},
			Searchables: []string{"User", "Group"},
			Requireds:   []string{"User", "Group", "Role"},
			AccessCheck: superAccessCheck,
			Attributes: map[string]AdminAttribute{
				"Role": {
					Default: GroupRoleMember,
					Choices: []AdminSelectOption{{"Admin", GroupRoleAdmin}, {"Member", GroupRoleMember}},
				},
			},
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
		obj.Path = path.Join(r.BasePath(), obj.Path) + "/"
		for idx := range obj.Fields {
			f := &obj.Fields[idx]
			if f.Foreign == nil {
				continue
			}
			f.Foreign.Path = path.Join(r.BasePath(), f.Foreign.Path) + "/"
		}

		obj.RegisterAdmin(objr)
		handledObjects = append(handledObjects, obj)
	}

	r.POST("/admin.json", func(ctx *gin.Context) {
		handleAdminIndex(ctx, handledObjects)
	})
	r.StaticFS("/", NewCombineEmbedFS(adminAssetsRoot, "admin", embedAdminAssets))
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
	siteCtx := GetRenderPageContext(c)
	db := getDbConnection(c, nil, false)
	siteCtx["dashboard"] = GetValue(db, KEY_ADMIN_DASHBOARD)
	c.JSON(http.StatusOK, gin.H{
		"objects": viewObjects,
		"user":    CurrentUser(c),
		"site":    siteCtx,
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
	r.POST("/:name", obj.handleAction)
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

	obj.ignores = map[string]bool{}
	err := obj.parseFields(db, rt)
	if err != nil {
		return err
	}
	if len(obj.PrimaryKey) <= 0 {
		return fmt.Errorf("%s not has primaryKey", obj.Name)
	}

	for idx := range obj.Actions {
		action := &obj.Actions[idx]
		if action.Name == "" {
			continue
		}
		if action.Path == "" {
			action.Path = strings.ToLower(action.Name)
		}
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

		if strings.Contains(gormTag, "primarykey") {
			field.Primary = true
			if strings.Contains(field.Type, "int") {
				field.IsAutoID = true
			}
		}

		if strings.Contains(gormTag, "primarykey") || strings.Contains(gormTag, "unique") {
			obj.PrimaryKey = append(obj.PrimaryKey, field.Name)
		}

		foreignKey := ""
		// ignore `belongs to` and `has one` relation
		if f.Type.Kind() == reflect.Struct || (f.Type.Kind() == reflect.Ptr && f.Type.Elem().Kind() == reflect.Struct) {
			hintForeignKey := fmt.Sprintf("%sID", field.Type)
			if _, ok := rt.FieldByName(hintForeignKey); ok {
				foreignKey = hintForeignKey
			}
		}
		if strings.Contains(gormTag, "foreignkey") {
			//extract foreign key from gorm tag with regex
			//example: foreignkey:UserRefer
			var re = regexp.MustCompile(`foreignkey:([a-zA-Z0-9]+)`)
			matches := re.FindStringSubmatch(gormTag)
			if len(matches) > 1 {
				foreignKey = strings.TrimSpace(matches[1])
			}
		}

		if foreignKey != "" {
			obj.ignores[foreignKey] = true
			for k := range obj.Fields {
				if obj.Fields[k].fieldName == foreignKey {
					// remove foreign key from fields
					obj.Fields = append(obj.Fields[:k], obj.Fields[k+1:]...)
					break
				}
			}

			field.Foreign = &AdminForeign{
				Field:      db.NamingStrategy.ColumnName(obj.tableName, foreignKey),
				Path:       strings.ToLower(f.Type.Name()),
				foreignKey: foreignKey,
				fieldName:  f.Name,
			}
		}

		if field.Type == "NullTime" || field.Type == "Time" {
			field.Type = "datetime"
		}

		if attr, ok := obj.Attributes[f.Name]; ok {
			field.Attribute = &attr
		}
		obj.Fields = append(obj.Fields, field)
	}
	return nil
}

func convertValue(elemType reflect.Type, source any) (any, error) {
	srcType := reflect.TypeOf(source)
	if srcType == elemType {
		return source, nil
	}
	var targetType reflect.Type = elemType
	var err error
	switch elemType.Name() {
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
		// if primary key in editables, then ignore it
		if _, ok := vals[k]; !ok {
			vals[k] = v
		}
	}

	for _, field := range obj.Fields {
		val, ok := vals[field.Name]
		if !ok {
			continue
		}

		if val == nil {
			continue
		}
		var target reflect.Value
		var targetValue reflect.Value
		var targetType = field.elemType
		if field.Foreign != nil {
			target = elemObj.Elem().FieldByName(field.Foreign.foreignKey)
			targetType = target.Type()
		} else {
			target = elemObj.Elem().FieldByName(field.fieldName)
		}

		fieldValue, err := convertValue(targetType, val)
		if err != nil {
			return nil, fmt.Errorf("invalid type: %s except: %s actual: %s error:%v", field.Name, field.Type, reflect.TypeOf(val).Name(), err)
		}
		targetValue = reflect.ValueOf(fieldValue)

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
		var fieldVal any
		if field.Foreign != nil {
			v := AdminValue{
				Value: rv.FieldByName(field.Foreign.foreignKey).Interface(),
			}
			fv := rv.FieldByName(field.Foreign.fieldName)
			if fv.IsValid() {
				if sv, ok := fv.Interface().(fmt.Stringer); ok {
					v.Label = sv.String()
				}
			}
			if v.Label == "" {
				v.Label = fmt.Sprintf("%v", v.Value)
			}
			fieldVal = v
		} else {
			v := rv.FieldByName(field.fieldName)
			if v.IsValid() {
				fieldVal = v.Interface()
			}
		}
		result[field.Name] = fieldVal
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

	result := db.Preload(clause.Associations).Where(keys).First(modelObj)

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
		if v.Foreign != nil {
			selected = append(selected, v.Foreign.Field)
		} else {
			selected = append(selected, v.Name)
		}
	}

	vals := reflect.New(reflect.SliceOf(obj.modelElem))
	tx := db.Preload(clause.Associations).Select(selected).Offset(form.Pos)
	if form.Limit > 0 {
		tx = tx.Limit(form.Limit)
	}
	result := tx.Find(vals.Interface())
	if result.Error != nil {
		return r, result.Error
	}

	for i := 0; i < vals.Elem().Len(); i++ {
		modelObj := vals.Elem().Index(i).Interface()
		r.objects = append(r.objects, modelObj)
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

	if form.ForeignMode {
		form.Limit = 0 // TODO: support foreign mode limit
	}

	r, err := obj.QueryObjects(db, form, c)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}
	if form.ForeignMode {
		var items []map[string]any
		for i := 0; i < len(r.Items); i++ {
			item := map[string]any{}
			var valueVal any
			for _, v := range obj.Fields {
				if v.Primary {
					valueVal = r.Items[i][v.Name]
				}
			}
			if valueVal == nil {
				continue
			}
			item["value"] = valueVal
			iv := r.objects[i]
			if sv, ok := iv.(fmt.Stringer); ok {
				item["label"] = sv.String()
			} else {
				item["label"] = fmt.Sprintf("%v", valueVal)
			}
			items = append(items, item)
		}
		r.Items = items
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
		if err := obj.BeforeCreate(c, elmObj); err != nil {
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

	for _, action := range obj.Actions {
		if action.Path != c.Param("name") {
			continue
		}

		keys := obj.getPrimaryValues(c)
		if len(keys) <= 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "invalid primary key",
			})
			return
		}
		db := getDbConnection(c, obj.GetDB, false)
		modelObj := reflect.New(obj.modelElem).Interface()
		result := db.Where(keys).First(modelObj)

		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "not found"})
			} else {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
			}
			return
		}
		r, err := action.Handler(db, c, modelObj)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		c.JSON(http.StatusOK, r)
		return
	}
	c.AbortWithStatus(http.StatusBadRequest)
}
