package carrot

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestStaticAssets(t *testing.T) {
	r := gin.Default()
	db, err := InitDatabase(nil, "", "")
	assert.Nil(t, err)
	InitCarrot(db, r)

	client := NewTestClient(r)
	w := client.Get("/static/img/carrot.svg")
	assert.Equal(t, w.Code, http.StatusOK)
	w = client.Get("/static/img/carrot-bad.svg")
	assert.Equal(t, w.Code, http.StatusNotFound)
}
