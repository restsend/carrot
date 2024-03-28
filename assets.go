package carrot

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

//go:embed admin
var EmbedAdminAssets embed.FS

type CombineEmbedFS struct {
	embeds    []EmbedFS
	assertDir string
}
type EmbedFS struct {
	EmbedRoot string
	Embedfs   embed.FS
}

func NewCombineEmbedFS(assertDir string, es ...EmbedFS) *CombineEmbedFS {
	return &CombineEmbedFS{
		embeds:    es,
		assertDir: assertDir,
	}
}

func (c *CombineEmbedFS) Open(name string) (http.File, error) {
	if c.assertDir != "" {
		f, err := os.Open(filepath.Join(c.assertDir, name))
		if err == nil {
			return f, nil
		}
	}

	var err error
	var ef fs.File
	for _, efs := range c.embeds {
		ef, err = efs.Embedfs.Open(filepath.Join(efs.EmbedRoot, name))
		if err == nil {
			return EmbedFile{ef}, nil
		}
	}
	return EmbedFile{ef}, err
}

type EmbedFile struct {
	f fs.File
}

// Close implements http.File
func (ef EmbedFile) Close() error {
	return ef.f.Close()
}

// Read implements http.File
func (ef EmbedFile) Read(p []byte) (n int, err error) {
	return ef.f.Read(p)
}

// Seek implements http.File
func (ef EmbedFile) Seek(offset int64, whence int) (int64, error) {
	return offset, nil
}

// Readdir implements http.File
func (ef EmbedFile) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, nil
}

// Stat implements http.File
func (ef EmbedFile) Stat() (fs.FileInfo, error) {
	return ef.f.Stat()
}

func GetRenderPageContext(c *gin.Context) map[string]any {
	db := c.MustGet(DbField).(*gorm.DB)
	LoadAutoloads(db)
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
		"user_id_type":       GetValue(db, KEY_SITE_USER_ID_TYPE),
	}
}

func HintAssetsRoot(dirName string) string {
	for _, dir := range []string{".", ".."} {
		testDirName := filepath.Join(os.ExpandEnv(dir), dirName)
		st, err := os.Stat(testDirName)

		if err == nil && st.IsDir() {
			return testDirName
		}
	}
	return ""
}
