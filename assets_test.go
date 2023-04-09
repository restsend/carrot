package carrot

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestLocate(t *testing.T) {
	as := NewStaticAssets()
	as.Paths = []string{"assets/html"}
	assert.Contains(t, as.Locate("signin.html"), "assets/html")
}
func TestStaticAssets(t *testing.T) {
	as := NewStaticAssets()
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

	r.HTMLRender.(*StaticAssets).Paths = []string{"assets"}
	assert.Nil(t, err)

	client := NewTestClient(r)
	w := client.Get("/static/img/carrot.svg")
	assert.Equal(t, w.Code, http.StatusOK)
	w = client.Get("/static/img/carrot-bad.svg")
	assert.Equal(t, w.Code, http.StatusNotFound)
}
