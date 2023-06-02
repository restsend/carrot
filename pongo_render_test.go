package carrot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPongoRender(t *testing.T) {
	as := NewStaticAssets()
	as.Paths = []string{"assets"}
	{
		w := httptest.NewRecorder()
		err := as.Instance("index.html", map[string]any{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "Coming soon")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("privacy.html", map[string]any{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "about these policy")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("terms.html", map[string]any{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "about these Terms")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("signin.html", map[string]any{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "Sign in to your account")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("signup.html", map[string]any{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "Free Sign Up")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("reset_password.html", map[string]any{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "Reset Password")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("reset_password_done.html", map[string]any{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "Enter your new password")
	}
	{

		r := gin.Default()
		db, err := InitDatabase(nil, "", "")
		assert.Nil(t, err)
		err = InitCarrot(db, r)
		assert.Nil(t, err)
		r.GET("/", func(ctx *gin.Context) {
			data := GetRenderPageContext(ctx)
			data["sitename"] = "MOCK_TEST"
			ctx.HTML(http.StatusOK, "index.html", data)
		})
		r.HTMLRender.(*StaticAssets).Paths = []string{"assets"}
		client := NewTestClient(r)
		w := client.Get("/")
		assert.Equal(t, w.Code, http.StatusOK)
		assert.Contains(t, w.Body.String(), "MOCK_TEST")
		assert.Contains(t, w.Body.String(), "Coming soon")
	}
}

func TestCarrotFilters(t *testing.T) {
	RegisterCarrotFilters()
	tmpl := `{{objects|stringify}}`
	r, err := pongo2.DefaultSet.RenderTemplateString(tmpl, pongo2.Context{
		"objects": gin.H{
			"Group": "Sys",
			"Name":  "User",
		},
	})
	assert.Nil(t, err)
	assert.Contains(t, r, `"Group":"Sys","Name":"User"`)
}
