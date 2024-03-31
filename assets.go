package carrot

import (
	"embed"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"gorm.io/gorm"
)

//go:embed static
var EmbedStaticAssets embed.FS

//go:embed templates
var EmbedTemplates embed.FS

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

func (c *CombineEmbedFS) Open(name string) (fs.File, error) {
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

func (c *CombineEmbedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if c.assertDir != "" {
		f, err := os.ReadDir(filepath.Join(c.assertDir, name))
		if err == nil {
			return f, nil
		}
	}
	return nil, errors.New("not found")
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
	loginNext := c.Query("next")
	if loginNext == "" {
		loginNext = GetValue(db, KEY_SITE_LOGIN_NEXT)
	}

	return map[string]any{
		"LoginNext": loginNext,
		"Site": map[string]any{
			"Url":              GetValue(db, KEY_SITE_URL),
			"Name":             GetValue(db, KEY_SITE_NAME),
			"Admin":            GetValue(db, KEY_SITE_ADMIN),
			"Keywords":         GetValue(db, KEY_SITE_KEYWORDS),
			"Description":      GetValue(db, KEY_SITE_DESCRIPTION),
			"GA":               GetValue(db, KEY_SITE_GA),
			"LogoUrl":          GetValue(db, KEY_SITE_LOGO_URL),
			"FaviconUrl":       GetValue(db, KEY_SITE_FAVICON_URL),
			"TermsUrl":         GetValue(db, KEY_SITE_TERMS_URL),
			"PrivacyUrl":       GetValue(db, KEY_SITE_PRIVACY_URL),
			"SigninUrl":        GetValue(db, KEY_SITE_SIGNIN_URL),
			"SignupUrl":        GetValue(db, KEY_SITE_SIGNUP_URL),
			"LogoutUrl":        GetValue(db, KEY_SITE_LOGOUT_URL),
			"ResetPasswordUrl": GetValue(db, KEY_SITE_RESET_PASSWORD_URL),
			"UserIdType":       GetValue(db, KEY_SITE_USER_ID_TYPE),
		},
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

type CombineTemplates struct {
	CombineFS *CombineEmbedFS
	Template  *template.Template
	Delims    render.Delims
	FuncMap   template.FuncMap
}

func NewCombineTemplates(combineFS *CombineEmbedFS) *CombineTemplates {
	return &CombineTemplates{
		CombineFS: combineFS,
		Delims:    render.Delims{Left: "{{", Right: "}}"},
		FuncMap:   NewTemplateFuncs(),
	}
}

func NewTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"default": func(value string) string {
			return value
		},
	}
}

func SanitizeSensitiveValues(prefix string, data any) map[string]any {
	if data == nil {
		return nil
	}

	vals, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	outVals := make(map[string]any)
	lowKeyRe := regexp.MustCompile(`(?i)(password|salt|secret)$`)
	for k, v := range vals {
		if len(prefix) > 0 {
			k = prefix + "." + k
		}
		if subVals, ok := v.(map[string]any); ok {
			subVals = SanitizeSensitiveValues(k, subVals)
			for sk, sv := range subVals {
				outVals[sk] = sv
			}
			continue
		}
		if lowKeyRe.MatchString(k) {
			outVals[k] = "********"
		} else {
			outVals[k] = v
		}
	}
	return outVals
}

func formatSources(source string) []any {
	var sources []any
	lines := regexp.MustCompile(`\r?\n`).Split(source, -1)
	for i, line := range lines {
		sources = append(sources, map[string]any{
			"Num":  i + 1,
			"Text": line,
		})
	}
	return sources
}
func (c *CombineTemplates) RenderDebug(name, source string, data any, err error) render.Render {
	lineAt := 4
	ctx := map[string]any{
		"Name":    name,
		"Error":   err,
		"Context": SanitizeSensitiveValues("", data),
		"Message": err.Error(),
		"Sources": formatSources(source),
		"LineAt":  lineAt,
	}

	tmpl := `<html>
	<head>
		<title>Error</title>
	</head>
	<body>
		<style>
			body {
				font-family: Arial, sans-serif;
				padding: 20px;
			}
			h1 {
				color: #f00;
			}
			h2 {
				color: #f00;
			}
			.code {
				background-color: #f8f8f8;
				border: 1px solid #ddd;
				padding: 10px;
				overflow: auto;
				word-wrap: break-word;
			}
			.line {
				color: #f00;
			}
		</style>
		<h1>Error</h1>
		<p>An error occurred while rendering: {{.Name}}</p>
		{{if .Message}}
		<p>Error: {{.Message}}</p>
		{{end}}
		{{if .Sources}}
		<h2>Sources</h2>
		<div class="code">
{{range $line := .Sources}}
<p>
{{if eq $line.Num $.LineAt}}
<h5><span class="line">{{$line.Num}}</span> {{$line.Text}}</h5>
{{else}}
<span class="line">{{$line.Num}}</span> {{$line.Text}}
{{end}}
{{end}}
<p>
</div>
{{end}}
		
		{{if .Context}}
		<h2>Context</h2>
		<!--for each context-->
		<div>
		{{range $key, $value := .Context}}
		<p><strong>{{$key}}</strong>: <span>{{$value}}</span></p>
		{{end}}
		</div>
		{{end}}
	</body>
	</html>`

	if gin.Mode() == gin.DebugMode {
		debugTmplFile, err := c.CombineFS.Open(".debug.html")
		if err == nil {
			if tmplData, err := io.ReadAll(debugTmplFile); err == nil {
				tmpl = string(tmplData)
			}
		}
	}

	r := &render.HTML{
		Template: template.Must(template.New(name).Funcs(c.FuncMap).Delims(c.Delims.Left, c.Delims.Right).Parse(tmpl)),
		Name:     name,
		Data:     ctx,
	}
	return r
}

// gin.render.Render
func (c *CombineTemplates) Instance(name string, ctx any) render.Render {
	tmplFile, err := c.CombineFS.Open(name)
	if err != nil {
		return c.RenderDebug(name, "", ctx, err)
	}

	tmplData, err := io.ReadAll(tmplFile)
	if err != nil {
		return c.RenderDebug(name, "", ctx, err)
	}
	tmpl := string(tmplData)
	t, err := template.New(name).Funcs(c.FuncMap).Delims(c.Delims.Left, c.Delims.Right).Parse(tmpl)
	if err != nil {
		return c.RenderDebug(name, tmpl, ctx, err)
	}

	if c.FuncMap == nil {
		c.FuncMap = NewTemplateFuncs()
	}

	r := &render.HTML{
		Template: t,
		Name:     name,
		Data:     ctx,
	}
	return r
}

func WithStaticAssets(r *gin.Engine, staticPrefix, staticRootDir string) gin.HandlerFunc {
	if staticRootDir == "" {
		staticRootDir = "static"
	}
	staticAssets := NewCombineEmbedFS(HintAssetsRoot(staticRootDir), EmbedFS{"static", EmbedStaticAssets})
	if staticPrefix == "" {
		staticPrefix = "/static"
	}
	r.StaticFS(staticPrefix, http.FS(staticAssets))
	return func(ctx *gin.Context) {
		ctx.Set(AssetsField, staticAssets)
		ctx.Next()
	}
}

func WithTemplates(r *gin.Engine, templateRootDir string) gin.HandlerFunc {
	if templateRootDir == "" {
		templateRootDir = "templates"
	}
	templatesAssets := NewCombineEmbedFS(HintAssetsRoot(templateRootDir), EmbedFS{"templates", EmbedTemplates})
	r.HTMLRender = NewCombineTemplates(templatesAssets)

	return func(ctx *gin.Context) {
		ctx.Set(TemplatesField, templatesAssets)
		ctx.Next()
	}
}
