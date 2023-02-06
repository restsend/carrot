package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/restsend/carrot"
)

func main() {
	// dsn := "root:123456@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=True&loc=Local"
	// db, _ := carrot.InitDatabase(nil, "mysql", dsn)

	db, _ := carrot.InitDatabase(nil, "", "")

	r := gin.Default()
	if err := carrot.InitCarrot(db, r); err != nil {
		panic(err)
	}

	// Check Default Value
	carrot.CheckValue(db, carrot.KEY_SITE_NAME, "Your Name")

	// Connect user event, eg. Login, Create
	carrot.Sig().Connect(carrot.SigUserCreate, func(sender interface{}, params ...interface{}) {
		user := sender.(*carrot.User)
		log.Println("create user", user.GetVisibleName())
	})
	carrot.Sig().Connect(carrot.SigUserLogin, func(sender interface{}, params ...interface{}) {
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
