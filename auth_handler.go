package carrot

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
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
	Password string `json:"password" form:"password" binding:"required"`
	Email    string `json:"email" form:"email" binding:"required"`
	Token    string `json:"token" form:"token" binding:"required"`
}

type ChangeEmailDoneForm struct {
	Password string `json:"password" form:"password"`
	Email    string `json:"email" form:"email" binding:"required"`
	Token    string `json:"token" form:"token" binding:"required"`
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
	authRoutes.GET("change_email_done", handleUserChangeEmailDonePage)
	authRoutes.POST("change_email_done", handleUserChangeEmailDone)
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
	RenderJSON(c, http.StatusOK, user)
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
	db := c.MustGet(DbField).(*gorm.DB)

	if GetValue(db, KEY_SITE_SIGNUP_URL) == "" {
		AbortWithJSONError(c, http.StatusNotImplemented, ErrUserNotAllowSignup)
		return
	}

	var form RegisterUserForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	if IsExistsByEmail(db, form.Email) {
		AbortWithJSONError(c, http.StatusBadRequest, ErrEmailExists)
		return
	}

	user, err := CreateUser(db, form.Email, form.Password)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"form": form,
		}).WithError(err).Warn("user: create user failed")
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	vals := make(map[string]any)
	if form.DisplayName != "" {
		vals["DisplayName"] = form.DisplayName
	}
	if form.FirstName != "" {
		vals["FirstName"] = form.FirstName
	}
	if form.LastName != "" {
		vals["LastName"] = form.LastName
	}
	if form.Locale != "" {
		vals["Locale"] = form.Locale
	}
	if form.Timezone != "" {
		vals["Timezone"] = form.Timezone
	}
	if form.Source != "" {
		vals["Source"] = form.Source
	}

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
		logrus.WithFields(logrus.Fields{
			"vals":   vals,
			"userId": user.ID,
		}).WithError(err).Warn("user: update user fields failed")
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
	RenderJSON(c, http.StatusOK, r)
}

func handleUserSignin(c *gin.Context) {
	var form LoginForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}

	if form.AuthToken == "" && form.Email == "" {
		AbortWithJSONError(c, http.StatusBadRequest, ErrEmptyEmail)
		return
	}

	if form.Password == "" && form.AuthToken == "" {
		AbortWithJSONError(c, http.StatusBadRequest, ErrEmptyPassword)
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	var user *User
	var err error
	if form.Password != "" {
		user, err = GetUserByEmail(db, form.Email)
		if err != nil {
			AbortWithJSONError(c, http.StatusBadRequest, ErrUserNotExists)
			return
		}
		if !CheckPassword(user, form.Password) {
			AbortWithJSONError(c, http.StatusUnauthorized, ErrUnauthorized)
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
	RenderJSON(c, http.StatusOK, user)
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
	RenderJSON(c, http.StatusOK, gin.H{})
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
	RenderJSON(c, http.StatusOK, gin.H{"expired": expired})
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
		AbortWithJSONError(c, http.StatusForbidden, ErrForbidden)
		return
	}

	if strings.EqualFold(user.Email, form.Email) {
		AbortWithJSONError(c, http.StatusBadRequest, ErrSameEmail)
		return
	}
	db := c.MustGet(DbField).(*gorm.DB)

	_, err := GetUserByEmail(db, form.Email)
	if err == nil {
		AbortWithJSONError(c, http.StatusBadRequest, ErrEmailExists)
		return
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Activated {
		AbortWithJSONError(c, http.StatusUnauthorized, ErrNotActivated)
		return
	}

	expired := sendHashMail(db, user, SigUserChangeEmail, KEY_VERIFY_EMAIL_EXPIRED, "30m", c.ClientIP(), c.Request.UserAgent(), form.Email)
	RenderJSON(c, http.StatusOK, gin.H{"expired": expired})
}

func handleUserChangeEmailDonePage(c *gin.Context) {
	db := c.MustGet(DbField).(*gorm.DB)
	token := c.Query("token")
	if token == "" {
		AbortWithJSONError(c, http.StatusForbidden, ErrTokenRequired)
		return
	}

	ctx := GetRenderPageContext(c)
	user, err := DecodeHashToken(db, token, true)
	if err != nil {
		AbortWithJSONError(c, http.StatusForbidden, err)
		return
	}
	newemail, _ := url.QueryUnescape(c.Query("email"))
	if newemail == "" {
		AbortWithJSONError(c, http.StatusBadRequest, ErrEmailRequired)
		return
	}
	if user.Password != "" {
		err = ChangeUserEmail(db, user, newemail)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"userId": user.ID,
				"email":  newemail,
			}).WithError(err).Warn("user: change email failed")
			AbortWithJSONError(c, http.StatusForbidden, err)
			return
		}
		logrus.WithFields(logrus.Fields{
			"userId": user.ID,
			"email":  newemail,
		}).Info("user: change email success")
	}
	ctx["Next"] = c.Query("next")
	ctx["Email"] = newemail
	ctx["Token"] = token
	ctx["EmptyPassword"] = user.Password == ""
	c.HTML(http.StatusOK, "auth/change_email_done.html", ctx)

}

func handleUserChangeEmailDone(c *gin.Context) {
	var form ChangeEmailDoneForm
	if err := c.BindJSON(&form); err != nil {
		AbortWithJSONError(c, http.StatusBadRequest, err)
		return
	}
	db := c.MustGet(DbField).(*gorm.DB)
	user, err := DecodeHashToken(db, form.Token, true)
	if err != nil {
		AbortWithJSONError(c, http.StatusForbidden, err)
		return
	}

	if user.Password == "" && form.Password == "" {
		AbortWithJSONError(c, http.StatusBadRequest, ErrEmptyPassword)
		return
	}

	if !strings.Contains(form.Email, "@") {
		form.Email = base64.StdEncoding.EncodeToString([]byte(form.Email))
	}

	err = ChangeUserEmail(db, user, form.Email)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"userId":   user.ID,
			"email":    form.Email,
			"clientIp": c.ClientIP(),
		}).WithError(err).Warn("user: change email failed")
		AbortWithJSONError(c, http.StatusForbidden, err)
		return
	}

	if user.Password == "" {
		err = SetPassword(db, user, form.Password)
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"userId":   user.ID,
				"clientIp": c.ClientIP(),
			}).WithError(err).Warn("user: change password failed")
			AbortWithJSONError(c, http.StatusInternalServerError, err)
			return
		}
	}

	logrus.WithFields(logrus.Fields{
		"userId":   user.ID,
		"email":    form.Email,
		"clientIp": c.ClientIP(),
	}).Info("user: change email success")

	next := c.Query("next")
	Login(c, user)

	if next != "" {
		c.Redirect(http.StatusFound, next)
	} else {
		RenderJSON(c, http.StatusOK, true)
	}
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
		logrus.WithFields(logrus.Fields{
			"userId":   user.ID,
			"clientIp": c.ClientIP(),
		}).WithError(err).Warn("user: change password failed")
		AbortWithJSONError(c, http.StatusInternalServerError, err)
		return
	}
	RenderJSON(c, http.StatusOK, true)
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
		RenderJSON(c, http.StatusOK, gin.H{"expired": "30m"})
		return
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Activated {
		AbortWithJSONError(c, http.StatusUnauthorized, ErrNotActivated)
		return
	}

	expired := sendHashMail(db, user, SigUserResetPassword, KEY_RESET_PASSWD_EXPIRED, "30m", c.ClientIP(), c.Request.UserAgent(), "")
	RenderJSON(c, http.StatusOK, gin.H{"expired": expired})
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
		AbortWithJSONError(c, http.StatusUnauthorized, ErrNotActivated)
		return
	}

	err = SetPassword(db, user, form.Password)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"userId":   user.ID,
			"clientIp": c.ClientIP(),
		}).WithError(err).Warn("user: reset password failed")
		AbortWithJSONError(c, http.StatusInternalServerError, err)
		return
	}
	RenderJSON(c, http.StatusOK, true)
}
