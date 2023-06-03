package carrot

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"gorm.io/gorm"
)

func GetRenderPageContext(c *gin.Context) map[string]any {
	db := c.MustGet(DbField).(*gorm.DB)
	return map[string]any{
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
		"logout_url":         GetValue(db, KEY_SITE_LOGOUT_URL),
		"reset_password_url": GetValue(db, KEY_SITE_RESET_PASSWORD_URL),
		"login_next":         GetValue(db, KEY_SITE_LOGIN_NEXT),
		"slogan":             GetValue(db, KEY_SITE_SLOGAN),
	}
}

func HintAssetsRoot(dirName string) string {
	var p string
	for _, dir := range []string{".", ".."} {
		testDirName := filepath.Join(os.ExpandEnv(dir), dirName)
		st, err := os.Stat(testDirName)

		if err == nil && st.IsDir() {
			return testDirName
		}
	}
	return p
}

type StaticAssets struct {
	TemplateDir string
	pongosets   *pongo2.TemplateSet
}

func NewStaticAssets() *StaticAssets {
	r := &StaticAssets{
		TemplateDir: HintAssetsRoot("templates"),
	}
	r.pongosets = pongo2.NewSet("carrot", r)
	return r
}

func (as *StaticAssets) InitStaticAssets(r *gin.Engine) {
	staticPrefix := GetEnv(ENV_STATIC_ROOT)
	if staticPrefix == "" {
		staticPrefix = "/static"
	}
	staticDir := HintAssetsRoot("static")
	if staticDir != "" {
		Warning("static serving at", staticPrefix, "->", staticDir)
		r.StaticFS(staticPrefix, http.Dir(staticDir))
	}
}

// pongo2.TemplateLoader
func (as *StaticAssets) Abs(base, name string) string {
	testFileName := filepath.Join(as.TemplateDir, filepath.Base(name))
	_, err := os.Stat(testFileName)
	if err == nil {
		return testFileName
	}
	return ""
}

// pongo2.TemplateLoader Get returns an io.Reader where the template's content can be read from.
func (as *StaticAssets) Get(path string) (io.Reader, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf), nil
}

// gin.HTML Render
func (as *StaticAssets) Instance(name string, ctx any) render.Render {
	vals := ctx.(map[string]any)
	r := &PongoRender{
		sets:     as.pongosets,
		fileName: name,
		ctx:      vals,
	}
	return r
}
