package carrot

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	UserField      = "_carrot_uid"
	GroupField     = "_carrot_gid"
	DbField        = "_carrot_db"
	TzField        = "_carrot_tz"
	AssetsField    = "_carrot_assets"
	TemplatesField = "_carrot_templates"
)

type RegisterUserForm struct {
	Email       string `json:"email" binding:"required"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"displayName"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Locale      string `json:"locale"`
	Timezone    string `json:"timezone"`
	Source      string `json:"source"`
}

type LoginForm struct {
	Email     string `json:"email" comment:"Email address"`
	Password  string `json:"password,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
	Remember  bool   `json:"remember,omitempty"`
	AuthToken string `json:"token,omitempty"`
}

type ChangePasswordForm struct {
	Password string `json:"password" binding:"required"`
}

type ResetPasswordForm struct {
	Email string `json:"email" binding:"required"`
}

type ResetPasswordDoneForm struct {
	Password string `json:"password" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Token    string `json:"token" binding:"required"`
}

func InitAuthHandler(authRoutes gin.IRoutes) {
	authRoutes.GET("register", handleUserSignupPage)
	authRoutes.GET("login", handleUserSigninPage)
	authRoutes.GET("reset_password", handleUserResetPasswordPage)
	authRoutes.GET("reset_password_done", handleUserResetPasswordDonePage)

	authRoutes.GET("info", handleUserInfo)
	authRoutes.POST("register", handleUserSignup)
	authRoutes.POST("login", handleUserSignin)
	authRoutes.GET("logout", handleUserLogout)
	authRoutes.POST("resend", handleUserResendActivation)
	authRoutes.GET("activation", handleUserActivation)
	authRoutes.POST("change_password", handleUserChangePassword)
	authRoutes.POST("reset_password", handleUserResetPassword)
	authRoutes.POST("reset_password_done", handleUserResetPasswordDone)
	authRoutes.POST("change_email", handleUserChangeEmail)
	authRoutes.GET("change_email_done", handleUserChangeEmailDone)
}

func handleUserInfo(c *gin.Context) {
	user := CurrentUser(c)
	if user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	db := c.MustGet(DbField).(*gorm.DB)
	var err error
	user, err = GetUserByUID(db, user.ID)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	withToken := c.Query("with_token")
	if withToken != "" {
		expired, err := time.ParseDuration(withToken)
		if err == nil {
			if expired >= 24*time.Hour {
				expired = 24 * time.Hour
			}
			user.AuthToken = BuildAuthToken(db, user, expired, false)
		}
	}
	c.JSON(http.StatusOK, user)
}

func handleUserSignupPage(c *gin.Context) {
	ctx := GetRenderPageContext(c)
	ctx["SignupText"] = "Sign Up Now"
	c.HTML(http.StatusOK, "auth/signup.html", ctx)
}

func handleUserSigninPage(c *gin.Context) {
	ctx := GetRenderPageContext(c)
	c.HTML(http.StatusOK, "auth/signin.html", ctx)
}

func handleUserResetPasswordPage(c *gin.Context) {
	c.HTML(http.StatusOK, "auth/reset_password.html", GetRenderPageContext(c))
}

func handleUserResetPasswordDonePage(c *gin.Context) {
	db := c.MustGet(DbField).(*gorm.DB)
	token := c.Query("token")
	if token == "" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	ctx := GetRenderPageContext(c)
	user, err := DecodeHashToken(db, token, true)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	ctx["email"] = user.Email
	ctx["token"] = token
	c.HTML(http.StatusOK, "auth/reset_password_done.html", ctx)
}

func sendHashMail(db *gorm.DB, user *User, signame, expireKey, defaultExpired, clientIp, useragent, newemail string) string {
	d, err := time.ParseDuration(GetValue(db, expireKey))
	if err != nil {
		d, _ = time.ParseDuration(defaultExpired)
	}
	n := time.Now().Add(d)
	hash := EncodeHashToken(user, n.Unix(), true)
	// Send Mail
	//
	if newemail != "" {
		Sig().Emit(signame, user, hash, clientIp, useragent, newemail)
	} else {
		Sig().Emit(signame, user, hash, clientIp, useragent)
	}
	return d.String()
}

func handleUserSignup(c *gin.Context) {
	var form RegisterUserForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	if IsExistsByEmail(db, form.Email) {
		AbortWithJSONError(c, http.StatusBadRequest, errors.New("email has exists"))
		return
	}

	user, err := CreateUser(db, form.Email, form.Password)
	if err != nil {
		Warning("create user failed", form, err)
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	vals := StructAsMap(form, []string{
		"DisplayName",
		"FirstName",
		"LastName",
		"Locale",
		"Timezone",
		"Source"})

	n := time.Now().Truncate(1 * time.Second)
	vals["LastLogin"] = &n
	vals["LastLoginIP"] = c.ClientIP()

	user.DisplayName = form.DisplayName
	user.FirstName = form.FirstName
	user.LastName = form.LastName
	user.Locale = form.Locale
	user.Source = form.Source
	user.Timezone = form.Timezone
	user.LastLogin = &n
	user.LastLoginIP = c.ClientIP()

	err = UpdateUserFields(db, user, vals)
	if err != nil {
		Warning("update user fields fail id:", user.ID, vals, err)
	}

	Sig().Emit(SigUserCreate, user, c)

	r := gin.H{
		"email":      user.Email,
		"activation": user.Activated,
	}
	if !user.Activated && GetBoolValue(db, KEY_USER_ACTIVATED) {
		r["expired"] = sendHashMail(db, user, SigUserVerifyEmail, KEY_VERIFY_EMAIL_EXPIRED, "180d", c.ClientIP(), c.Request.UserAgent(), "")
	} else {
		Login(c, user) //Login now
	}
	c.JSON(http.StatusOK, r)
}

func handleUserSignin(c *gin.Context) {
	var form LoginForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	if form.AuthToken == "" && form.Email == "" {
		AbortWithJSONError(c, http.StatusBadRequest, errors.New("email is required"))
		return
	}

	if form.Password == "" && form.AuthToken == "" {
		AbortWithJSONError(c, http.StatusBadRequest, errors.New("empty password"))
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	var user *User
	var err error
	if form.Password != "" {
		user, err = GetUserByEmail(db, form.Email)
		if err != nil {
			AbortWithJSONError(c, http.StatusBadRequest, errors.New("user not exists"))
			return
		}
		if !CheckPassword(user, form.Password) {
			AbortWithJSONError(c, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
	} else {
		user, err = DecodeHashToken(db, form.AuthToken, false)
		if err != nil {
			AbortWithJSONError(c, http.StatusUnauthorized, err)
			return
		}
	}

	err = CheckUserAllowLogin(db, user)
	if err != nil {
		AbortWithJSONError(c, http.StatusForbidden, err)
		return
	}

	if form.Timezone != "" {
		InTimezone(c, form.Timezone)
	}

	Login(c, user)

	if form.Remember {
		val := GetValue(db, KEY_AUTH_TOKEN_EXPIRED) // 7d
		expired, err := time.ParseDuration(val)
		if err != nil {
			// 7 days
			expired = 7 * 24 * time.Hour
		}
		user.AuthToken = BuildAuthToken(db, user, expired, false)
	}
	c.JSON(http.StatusOK, user)
}

func handleUserLogout(c *gin.Context) {
	user := CurrentUser(c)
	if user != nil {
		Logout(c, user)
	}
	next := c.Query("next")
	if next != "" {
		c.Redirect(http.StatusFound, next)
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}

func handleUserResendActivation(c *gin.Context) {
	var form ResetPasswordForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	user, err := GetUserByEmail(db, form.Email)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	expired := "180d"
	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Activated {
		expired = sendHashMail(db, user, SigUserVerifyEmail, KEY_VERIFY_EMAIL_EXPIRED, "180d", c.ClientIP(), c.Request.UserAgent(), "")
	}
	c.JSON(http.StatusOK, gin.H{"expired": expired})
}

func handleUserActivation(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	next := c.Query("next")
	if next == "" {
		next = "/"
	}
	db := c.MustGet(DbField).(*gorm.DB)
	user, err := DecodeHashToken(db, token, true)
	if err != nil {
		AbortWithJSONError(c, http.StatusForbidden, err)
		return
	}

	user.Activated = true
	UpdateUserFields(db, user, map[string]any{
		"Activated": true,
	})

	InTimezone(c, user.Timezone)
	Login(c, user)
	c.Redirect(http.StatusFound, next)
}

func handleUserChangeEmail(c *gin.Context) {
	var form ResetPasswordForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}
	user := CurrentUser(c)
	if user == nil {
		AbortWithJSONError(c, http.StatusForbidden, errors.New("forbidden, please login"))
		return
	}

	if strings.EqualFold(user.Email, form.Email) {
		AbortWithJSONError(c, http.StatusBadRequest, errors.New("same email"))
		return
	}
	db := c.MustGet(DbField).(*gorm.DB)

	_, err := GetUserByEmail(db, form.Email)
	if err == nil {
		AbortWithJSONError(c, http.StatusBadRequest, errors.New("email has exists, please use another email"))
		return
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Activated {
		AbortWithJSONError(c, http.StatusUnauthorized, errors.New("waiting for activation"))
		return
	}

	expired := sendHashMail(db, user, SigUserChangeEmail, KEY_VERIFY_EMAIL_EXPIRED, "30m", c.ClientIP(), c.Request.UserAgent(), form.Email)
	c.JSON(http.StatusOK, gin.H{"expired": expired})
}

func handleUserChangeEmailDone(c *gin.Context) {
	db := c.MustGet(DbField).(*gorm.DB)
	token := c.Query("token")
	if token == "" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	email := c.Query("email")
	if email == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	user, err := DecodeHashToken(db, token, true)
	if err != nil {
		AbortWithJSONError(c, http.StatusForbidden, err)
		return
	}

	if !strings.Contains(email, "@") {
		email = base64.StdEncoding.EncodeToString([]byte(email))
	}

	err = ChangeUserEmail(db, user, email)
	if err != nil {
		AbortWithJSONError(c, http.StatusForbidden, err)
		return
	}

	next := c.Query("next")
	if next == "" {
		next = "/"
	}
	Login(c, user)
	c.Redirect(http.StatusFound, next)
}

func handleUserChangePassword(c *gin.Context) {
	var form ChangePasswordForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	user := CurrentUser(c)
	if user == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	err := CheckUserAllowLogin(db, user)
	if err != nil {
		AbortWithJSONError(c, http.StatusForbidden, err)
		return
	}
	err = SetPassword(db, user, form.Password)
	if err != nil {
		Warning("changed user password fail user:", user.ID, err.Error())
		AbortWithJSONError(c, http.StatusInternalServerError, errors.New("changed failed"))
		return
	}
	c.JSON(http.StatusOK, true)
}

func handleUserResetPassword(c *gin.Context) {
	var form ResetPasswordForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	user, err := GetUserByEmail(db, form.Email)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"expired": "30m"})
		return
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Activated {
		AbortWithJSONError(c, http.StatusUnauthorized, errors.New("waiting for activation"))
		return
	}

	expired := sendHashMail(db, user, SigUserResetPassword, KEY_RESET_PASSWD_EXPIRED, "30m", c.ClientIP(), c.Request.UserAgent(), "")
	c.JSON(http.StatusOK, gin.H{"expired": expired})
}

func handleUserResetPasswordDone(c *gin.Context) {
	var form ResetPasswordDoneForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}
	db := c.MustGet(DbField).(*gorm.DB)

	user, err := DecodeHashToken(db, form.Token, true)
	if err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	if !strings.EqualFold(user.Email, form.Email) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Activated {
		AbortWithJSONError(c, http.StatusUnauthorized, errors.New("waiting for activation"))
		return
	}

	err = SetPassword(db, user, form.Password)
	if err != nil {
		Warning("reset user password fail user:", user.ID, err.Error())
		AbortWithJSONError(c, http.StatusInternalServerError, errors.New("reset failed"))
		return
	}
	c.JSON(http.StatusOK, true)
}
