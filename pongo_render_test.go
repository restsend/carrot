package carrot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLocate(t *testing.T) {
	as := NewPongoAssets()
	as.Paths = []string{"assets/html"}
	assert.Contains(t, as.Locate("signin.html"), "assets/html")
}
func TestStaticAssets(t *testing.T) {
	as := NewPongoAssets()
	as.Paths = []string{"assets"}
	{
		_, err := as.Open("/img/carrot.svg")
		assert.Nil(t, err)
	}
	{
		_, err := as.Open("/img/file-not-exist")
		assert.NotNil(t, err)
	}
	{
		_, err := as.Open("/img/../../../../etc/initrc")
		assert.NotNil(t, err)
	}
	r := gin.Default()
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	err = InitCarrot(db, r)
	assert.Nil(t, err)

	client := NewTestClient(r)
	w := client.Get("/static/img/carrot.svg")
	assert.Equal(t, w.Code, http.StatusOK)
	w = client.Get("/static/img/carrot-bad.svg")
	assert.Equal(t, w.Code, http.StatusNotFound)
}
func TestPongoRender(t *testing.T) {
	as := NewPongoAssets()
	as.Paths = []string{"assets"}
	{
		w := httptest.NewRecorder()
		err := as.Instance("index.html", map[string]interface{}{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "Coming soon")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("privacy.html", map[string]interface{}{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "about these policy")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("terms.html", map[string]interface{}{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "about these Terms")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("signin.html", map[string]interface{}{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "Sign in to your account")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("signup.html", map[string]interface{}{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "Free Sign Up")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("reset_password.html", map[string]interface{}{
			"sitename": "test",
		}).Render(w)
		assert.Nil(t, err)
		assert.Contains(t, w.Body.String(), "Reset Password")
	}
	{
		w := httptest.NewRecorder()
		err := as.Instance("reset_password_done.html", map[string]interface{}{
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
		client := NewTestClient(r)
		w := client.Get("/")
		assert.Equal(t, w.Code, http.StatusOK)
		assert.Contains(t, w.Body.String(), "MOCK_TEST")
		assert.Contains(t, w.Body.String(), "Coming soon")
	}
}
