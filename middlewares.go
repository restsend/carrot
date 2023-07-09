package carrot

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AbortWithJSONError(c *gin.Context, code int, err error) {
	var errWithFileNum error = err
	if log.Flags()&(log.Lshortfile|log.Llongfile) != 0 {
		var ok bool
		_, file, line, ok := runtime.Caller(1)
		if !ok {
			file = "???"
			line = 0
		}
		pos := strings.LastIndex(file, "/")
		if log.Flags()&log.Lshortfile != 0 && pos >= 0 {
			file = file[1+pos:]
		}
		errWithFileNum = fmt.Errorf("%s:%d: %v", file, line, err)
	}
	c.Error(errWithFileNum)
	c.AbortWithStatusJSON(code, gin.H{"error": err.Error()})
}

func CORSEnabled() gin.HandlerFunc {
	return WithCORS(CORS_ALLOW_ALL, CORS_ALLOW_CREDENTIALS, CORS_ALLOW_HEADERS, CORS_ALLOW_METHODS)
}

func WithCORS(origin, credentials, headers, methods string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", credentials)
		c.Writer.Header().Set("Access-Control-Allow-Headers", headers)
		c.Writer.Header().Set("Access-Control-Allow-Methods", methods)

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent) // 204
			return
		}
		c.Next()
	}
}

func WithCookieSession(secret string) gin.HandlerFunc {
	store := cookie.NewStore([]byte(secret))
	store.Options(sessions.Options{Path: "/", MaxAge: 0})
	return sessions.Sessions(SessionField, store)
}

func WithMemSession(secret string) gin.HandlerFunc {
	store := memstore.NewStore([]byte(secret))
	store.Options(sessions.Options{Path: "/", MaxAge: 0})
	return sessions.Sessions(SessionField, store)
}

func WithGormDB(db *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(DbField, db)
		ctx.Next()
	}
}
