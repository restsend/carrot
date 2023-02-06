package carrot

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"gorm.io/gorm"
)

func GetRenderPageContext(c *gin.Context) map[string]interface{} {
	db := c.MustGet(DbField).(*gorm.DB)
	return map[string]interface{}{
		"siteurl":            GetValue(db, KEY_SITE_URL),
		"sitename":           GetValue(db, KEY_SITE_NAME),
		"copyright":          GetValue(db, KEY_SITE_COPYRIGHT),
		"siteadmin":          GetValue(db, KEY_SITE_ADMIN),
		"keywords":           GetValue(db, KEY_SITE_KEYWORDS),
		"description":        GetValue(db, KEY_SITE_DESCRIPTION),
		"ga":                 GetValue(db, KEY_SITE_GA),
		"logo_url":           GetValue(db, KEY_SITE_LOGO_URL),
		"favicon_url":        GetValue(db, KEY_SITE_FAVICON_URL),
		"terms_url":          GetValue(db, KEY_SITE_TERMS_URL),
		"privacy_url":        GetValue(db, KEY_SITE_PRIVACY_URL),
		"signin_url":         GetValue(db, KEY_SITE_SIGNIN_URL),
		"signup_url":         GetValue(db, KEY_SITE_SIGNUP_URL),
		"reset_password_url": GetValue(db, KEY_SITE_RESET_PASSWORD_URL),
		"login_next":         GetValue(db, KEY_SITE_LOGIN_NEXT),
		"slogan":             GetValue(db, KEY_SITE_SLOGAN),
	}
}

func HintAssetsRoot(paths []string) string {
	var p string
	for _, dir := range paths {
		testDirName := filepath.Join(os.ExpandEnv(dir), "assets")
		st, err := os.Stat(testDirName)

		if err == nil && st.IsDir() {
			return testDirName
		}
	}
	return p
}

type StaticAssets struct {
	Paths       []string
	TemplateDir string
	sets        *pongo2.TemplateSet
}

func NewStaticAssets() *StaticAssets {
	r := &StaticAssets{
		TemplateDir: "html",
	}
	r.sets = pongo2.NewSet("carrot", r)
	return r
}

func (as *StaticAssets) InitStaticAssets(r *gin.Engine) {
	staticPrefix := GetEnv(ENV_STATIC_ROOT)
	if staticPrefix == "" {
		staticPrefix = "/static"
	}

	r.StaticFS(staticPrefix, as)
}

func (as *StaticAssets) Locate(name string) string {
	for _, dir := range as.Paths {
		dir, _ = filepath.Abs(os.ExpandEnv(dir))
		testFileName := filepath.Join(dir, filepath.FromSlash(name))
		st, err := os.Stat(testFileName)

		if err == nil && !st.IsDir() {
			return testFileName
		}
	}
	return name
}

// pongo2.TemplateLoader
func (as *StaticAssets) Abs(base, name string) string {
	if base != "" {
		name = filepath.Join(as.TemplateDir, name)
	}
	return as.Locate(name)
}

// pongo2.TemplateLoader Get returns an io.Reader where the template's content can be read from.
func (as *StaticAssets) Get(path string) (io.Reader, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf), nil
}

// gin.StaticFS interface
func (as *StaticAssets) Open(name string) (http.File, error) {
	dir := filepath.Dir(name)
	if !strings.HasPrefix(dir, "/") || strings.ContainsRune(dir, '.') {
		return nil, errors.New("http: invalid character in file path")
	}
	name = as.Locate(name)
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return f, err
}

// gin.HTML Render
func (as *StaticAssets) Instance(name string, ctx any) render.Render {
	vals := ctx.(map[string]interface{})
	r := &PongoRender{
		sets:     as.sets,
		fileName: as.Locate(filepath.Join(as.TemplateDir, name)),
		ctx:      vals,
	}
	return r
}
