package carrot

// type AdminObject struct{}

// func RegisterAdmin(r gin.IRoutes, objs []AdminObject) {
// 	r.GET("/", func(ctx *gin.Context) {
// 		ctx.Data(http.StatusOK, "plain/text", []byte("ok"))
// 	})

// 	authr := r.Use(func(ctx *gin.Context) {
// 		user := CurrentUser(ctx)
// 		if user == nil || !user.IsStaff {
// 			ctx.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
// 			return
// 		}
// 	})
// 	_ = authr
// }
