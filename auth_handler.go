package carrot

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	UserField  = "_carrot_uid"
	GroupField = "_carrot_gid"
	DbField    = "_carrot_db"
	TzField    = "_carrot_tz"
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
	Email     string `json:"email"`
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

func InitAuthHandler(prefix string, db *gorm.DB, r *gin.Engine) {
	if prefix == "" {
		prefix = GetEnv(ENV_AUTH_PREFIX)
	}

	r.GET(filepath.Join(prefix, "register"), handleUserSignupPage)
	r.GET(filepath.Join(prefix, "login"), handleUserSigninPage)
	r.GET(filepath.Join(prefix, "reset_password"), handleUserResetPasswordPage)
	r.GET(filepath.Join(prefix, "reset_password_done"), handleUserResetPasswordDonePage)

	r.GET(filepath.Join(prefix, "info"), handleUserInfo)
	r.POST(filepath.Join(prefix, "register"), handleUserSignup)
	r.POST(filepath.Join(prefix, "login"), handleUserSignin)
	r.GET(filepath.Join(prefix, "logout"), handleUserLogout)
	r.POST(filepath.Join(prefix, "resend"), handleUserResendActivation)
	r.GET(filepath.Join(prefix, "activation"), handleUserActivation)
	r.POST(filepath.Join(prefix, "change_password"), handleUserChangePassword)
	r.POST(filepath.Join(prefix, "reset_password"), handleUserResetPassword)
	r.POST(filepath.Join(prefix, "reset_password_done"), handleUserResetPasswordDone)
}

func handleUserInfo(c *gin.Context) {
	user := CurrentUser(c)
	if user == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	db := c.MustGet(DbField).(*gorm.DB)
	var err error
	user, err = GetUserByUID(db, user.ID)
	if err != nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.JSON(http.StatusOK, user)
}

func handleUserSignupPage(c *gin.Context) {
	ctx := GetRenderPageContext(c)
	loginNext := c.Query("next")
	if loginNext != "" {
		ctx["login_next"] = loginNext
	}
	c.HTML(http.StatusOK, "signup.html", ctx)
}

func handleUserSigninPage(c *gin.Context) {
	ctx := GetRenderPageContext(c)
	loginNext := c.Query("next")
	if loginNext != "" {
		ctx["login_next"] = loginNext
	}
	c.HTML(http.StatusOK, "signin.html", ctx)
}

func handleUserResetPasswordPage(c *gin.Context) {
	c.HTML(http.StatusOK, "reset_password.html", GetRenderPageContext(c))
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
	c.HTML(http.StatusOK, "reset_password_done.html", ctx)
}

func sendHashMail(db *gorm.DB, user *User, signame, expireKey, defaultExpired, clientIp, useragent string) {
	d, err := time.ParseDuration(GetValue(db, expireKey))
	if err != nil {
		d, _ = time.ParseDuration(defaultExpired)
	}
	n := time.Now().Add(d)
	hash := EncodeHashToken(user, n.Unix(), true)
	// Send Mail
	//
	Sig().Emit(signame, user, hash, clientIp, useragent)
}

func handleUserSignup(c *gin.Context) {
	var form RegisterUserForm
	if err := c.BindJSON(&form); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	if IsExistsByEmail(db, form.Email) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "email has exists",
		})
		return
	}

	user, err := CreateUser(db, form.Email, form.Password)
	if err != nil {
		Warning("create user fail", form, err)
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "server error",
		})
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
		"activation": user.Actived,
	}
	if !user.Actived && GetBoolValue(db, KEY_USER_ACTIVATED) {
		sendHashMail(db, user, SigUserVerifyEmail, KEY_VERIFY_EMAIL_EXPIRED, "180d", c.ClientIP(), c.Request.UserAgent())
		r["expired"] = "180d"
	} else {
		Login(c, user) //Login now
	}
	c.JSON(http.StatusOK, r)
}

func handleUserSignin(c *gin.Context) {
	var form LoginForm
	if err := c.BindJSON(&form); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if form.AuthToken == "" && form.Email == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "email is required",
		})
		return
	}

	if form.Password == "" && form.AuthToken == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "empty password",
		})
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	var user *User
	var err error
	if form.Password != "" {
		user, err = GetUserByEmail(db, form.Email)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "user not exists",
			})
			return
		}
		if !CheckPassword(user, form.Password) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
			})
			return
		}
	} else {
		user, err = DecodeHashToken(db, form.AuthToken, false)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	if !user.Enabled {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "user not allow login",
		})
		return
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Actived {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "waiting for activation",
		})
		return
	}

	if form.Timezone != "" {
		InTimezone(c, form.Timezone)
	}

	Login(c, user)

	if form.Remember {
		// 7 days
		n := time.Now().Add(7 * 24 * time.Hour)
		user.AuthToken = EncodeHashToken(user, n.Unix(), false)
	}

	c.JSON(http.StatusOK, user)
}

func handleUserLogout(c *gin.Context) {
	user := CurrentUser(c)
	if user != nil {
		Logout(c, user)
	}
	c.JSON(http.StatusOK, gin.H{})
}

func handleUserResendActivation(c *gin.Context) {
	var form ResetPasswordForm
	if err := c.BindJSON(&form); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	user, err := GetUserByEmail(db, form.Email)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Actived {
		sendHashMail(db, user, SigUserVerifyEmail, KEY_VERIFY_EMAIL_EXPIRED, "180d", c.ClientIP(), c.Request.UserAgent())
	}
	c.JSON(http.StatusOK, gin.H{"expired": "180d"})
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
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": err.Error(),
		})
		return
	}

	user.Actived = true
	UpdateUserFields(db, user, map[string]any{
		"Actived": true,
	})

	InTimezone(c, user.Timezone)
	Login(c, user)
	c.Redirect(http.StatusFound, next)
}

func handleUserChangePassword(c *gin.Context) {
	var form ChangePasswordForm
	if err := c.BindJSON(&form); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	user := CurrentUser(c)
	if user == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	if !user.Enabled {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "user not allow login",
		})
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Actived {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "waiting for activation",
		})
		return
	}
	err := SetPassword(db, user, form.Password)
	if err != nil {
		Warning("changed user password fail user:", user.ID, err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "changed fail",
		})
		return
	}
	c.JSON(http.StatusOK, true)
}

func handleUserResetPassword(c *gin.Context) {
	var form ResetPasswordForm
	if err := c.BindJSON(&form); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	user, err := GetUserByEmail(db, form.Email)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"expired": "30m"})
		return
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Actived {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "waiting for activation",
		})
		return
	}

	sendHashMail(db, user, SigUserResetpassword, KEY_RESET_PASSWD_EXPIRED, "30m", c.ClientIP(), c.Request.UserAgent())
	c.JSON(http.StatusOK, gin.H{"expired": "30m"})
}

func handleUserResetPasswordDone(c *gin.Context) {
	var form ResetPasswordDoneForm
	if err := c.BindJSON(&form); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}
	db := c.MustGet(DbField).(*gorm.DB)

	user, err := DecodeHashToken(db, form.Token, true)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if !strings.EqualFold(user.Email, form.Email) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Actived {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "waiting for activation",
		})
		return
	}

	err = SetPassword(db, user, form.Password)
	if err != nil {
		Warning("reset user password fail user:", user.ID, err.Error())
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "reset fail",
		})
		return
	}
	c.JSON(http.StatusOK, true)
}
