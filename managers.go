package carrot

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Manager interface {
	Migrate(db *gorm.DB) error
	RegisterHandler(r *gin.Engine) error
}
