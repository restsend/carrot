package main

import (
	"github.com/gin-gonic/gin"
	"github.com/restsend/carrot"
)

func main() {
	db, _ := carrot.InitDatabase(nil, "", "")

	r := gin.Default()
	if err := carrot.InitCarrot(db, r); err != nil {
		panic(err)
	}

	if as, ok := r.HTMLRender.(*carrot.StaticAssets); ok {
		paths := []string{carrot.HintAssetsRoot([]string{"./", "../"})}
		as.Paths = append(paths, as.Paths...)
	}

	// Check Default Value
	carrot.CheckValue(db, carrot.KEY_SITE_NAME, "Carrot")

	// Connect user event, eg. Login, Create
	carrot.Sig().Connect(carrot.SigUserCreate, func(sender any, params ...any) {
		user := sender.(*carrot.User)
		carrot.Info("create user: ", user.GetVisibleName())
	})
	carrot.Sig().Connect(carrot.SigUserLogin, func(sender any, params ...any) {
		user := sender.(*carrot.User)
		carrot.Info("user logined: ", user.GetVisibleName())
	})

	// http://localhost:8080/auth/login
	r.Run(":8080")
}
