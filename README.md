carrot - restsend golang library
====
# Quick start
```go
package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/restsend/carrot"
)

func main() {
	db, _ := carrot.InitDatabase(nil, "", "example.db")
	r := gin.Default()
	if err := carrot.InitCarrot(db, r); err != nil {
		panic(err)
	}

	// Check Default Value
	carrot.CheckValue(db, carrot.KEY_SITE_NAME, "Your Name")

	// Connect user event, eg. Login, Create
	carrot.Sig().Connect(carrot.SigUserCreate, func(sender any, params ...any) {
		user := sender.(*carrot.User)
		log.Println("create user", user.GetVisibleName())
	})
	carrot.Sig().Connect(carrot.SigUserLogin, func(sender any, params ...any) {
		user := sender.(*carrot.User)
		log.Println("user logined", user.GetVisibleName())
	})

	r.GET("/", func(ctx *gin.Context) {
		data := carrot.GetRenderPageContext(ctx)
		data["title"] = "Welcome"
		ctx.HTML(http.StatusOK, "index.html", data)
	})
	// Visit:
	//  http://localhost:8080/
	//  http://localhost:8080/auth/login
	r.Run(":8080")
}
```

# Users - builtin user managment

# WebObject - gorm with restful api

# Static assets - multi path loader static assets

# Signals 
