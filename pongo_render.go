package carrot

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
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

type PongoAssets struct {
	Paths           []string
	TemplateDir     string
	StaticAssetsDir []string
	sets            *pongo2.TemplateSet
}

func NewPongoAssets() *PongoAssets {
	r := &PongoAssets{
		TemplateDir:     "html",
		StaticAssetsDir: []string{"img", "css", "fonts", "js"},
	}
	r.sets = pongo2.NewSet("carrot", r)
	return r
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

func (as *PongoAssets) InitStaticAssets(r *gin.Engine) {
	staticPrefix := GetEnv(ENV_STATIC_ROOT)
	if staticPrefix == "" {
		staticPrefix = "/static"
	}

	r.StaticFS(staticPrefix, as)

	p := HintAssetsRoot([]string{".", "..", "../carrot"})
	if p != "" {
		as.Paths = append(as.Paths, p)
	}
}

// HTML Render
func (as *PongoAssets) Instance(name string, ctx any) render.Render {
	vals := ctx.(map[string]interface{})
	r := &PongoRender{
		as:       as,
		fileName: as.Locate(filepath.Join(as.TemplateDir, name)),
		ctx:      vals,
	}
	return r
}

// Static
func (as *PongoAssets) Open(name string) (http.File, error) {
	dir := filepath.Dir(name)
	if !strings.HasPrefix(dir, "/") || strings.ContainsRune(dir, '.') {
		return nil, errors.New("http: invalid character in file path")
	}
	dir = dir[1:]
	hint := false
	for _, v := range as.StaticAssetsDir {
		if v == dir {
			hint = true
			break
		}
	}
	if !hint {
		return nil, fs.ErrPermission
	}
	return os.Open(as.Locate(filepath.Join(dir, filepath.Base(name))))
}

func (as *PongoAssets) Abs(base, name string) string {
	if base != "" {
		name = filepath.Join(as.TemplateDir, name)
	}
	return as.Locate(name)
}

// Get returns an io.Reader where the template's content can be read from.
func (as *PongoAssets) Get(path string) (io.Reader, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(buf), nil
}

func (as *PongoAssets) Locate(name string) string {
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

type PongoRender struct {
	as       *PongoAssets
	fileName string
	ctx      map[string]interface{}
}

// Render implements render.Render
func (r *PongoRender) Render(w http.ResponseWriter) error {
	r.WriteContentType(w)
	t, err := r.as.sets.FromFile(r.fileName)
	if err != nil {
		return err
	}
	result, err := t.ExecuteBytes(r.ctx)
	if err != nil {
		return err
	}
	_, err = w.Write(result)
	return err
}

// WriteContentType implements render.Render
func (r *PongoRender) WriteContentType(w http.ResponseWriter) {
	header := w.Header()
	if val := header["Content-Type"]; len(val) == 0 {
		header["Content-Type"] = []string{"text/html; charset=utf-8"}
	}
}
