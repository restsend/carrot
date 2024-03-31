package carrot

import (
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Gin session field name
const ENV_SESSION_FIELD = "SESSION_FIELD"

// Default Value: 1024
const ENV_CONFIG_CACHE_SIZE = "CONFIG_CACHE_SIZE"

// Default Value: 10s
const ENV_CONFIG_CACHE_EXPIRED = "CONFIG_CACHE_EXPIRED"

// DB
const ENV_DB_DRIVER = "DB_DRIVER"
const ENV_DSN = "DSN"
const ENV_SESSION_SECRET = "SESSION_SECRET"

// User Password salt
const ENV_SALT = "PASSWORD_SALT"
const ENV_AUTH_PREFIX = "AUTH_PREFIX"
const ENV_STATIC_PREFIX = "STATIC_PREFIX"
const ENV_STATIC_ROOT = "STATIC_ROOT"
const ENV_TEMPLATE_ROOT = "TEMPLATE_ROOT"

const KEY_USER_ACTIVATED = "USER_ACTIVATED"
const KEY_AUTH_TOKEN_EXPIRED = "AUTH_TOKEN_EXPIRED"
const KEY_RESET_PASSWD_EXPIRED = "RESET_PASSWD_EXPIRED"
const KEY_VERIFY_EMAIL_EXPIRED = "VERIFY_EMAIL_EXPIRED"

const KEY_SITE_NAME = "SITE_NAME"
const KEY_SITE_ADMIN = "SITE_ADMIN"
const KEY_SITE_URL = "SITE_URL"
const KEY_SITE_KEYWORDS = "SITE_KEYWORDS"
const KEY_SITE_DESCRIPTION = "SITE_DESCRIPTION"
const KEY_SITE_GA = "SITE_GA"
const KEY_SITE_COPYRIGHT = "SITE_COPYRIGHT"
const KEY_SITE_LOGO_URL = "SITE_LOGO_URL"
const KEY_SITE_FAVICON_URL = "SITE_FAVICON_URL"
const KEY_SITE_TERMS_URL = "SITE_TERMS_URL"
const KEY_SITE_PRIVACY_URL = "SITE_PRIVACY_URL"
const KEY_SITE_SIGNIN_URL = "SITE_SIGNIN_URL"
const KEY_SITE_SIGNUP_URL = "SITE_SIGNUP_URL"
const KEY_SITE_LOGOUT_URL = "SITE_LOGOUT_URL"
const KEY_SITE_RESET_PASSWORD_URL = "SITE_RESET_PASSWORD_URL"
const KEY_SITE_LOGIN_NEXT = "SITE_LOGIN_NEXT"
const KEY_SITE_USER_ID_TYPE = "SITE_USER_ID_TYPE"

// Cors
const CORS_ALLOW_ALL = "*"
const CORS_ALLOW_CREDENTIALS = "true"
const CORS_ALLOW_HEADERS = "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Auth-Token"
const CORS_ALLOW_METHODS = "POST, OPTIONS, GET, PUT, PATCH, DELETE"

func InitCarrot(db *gorm.DB, r *gin.Engine) (err error) {
	err = InitMigrate(db)
	if err != nil {
		log.Fatal("migrate failed", err)
	}

	r.Use(WithGormDB(db), CORSEnabled())
	r.Use(WithStaticAssets(r, GetEnv(ENV_STATIC_PREFIX), GetEnv(ENV_STATIC_ROOT)))
	r.Use(WithTemplates(r, GetEnv(ENV_TEMPLATE_ROOT)))

	secret := GetEnv(ENV_SESSION_SECRET)
	if secret != "" {
		r.Use(WithCookieSession(secret))
	} else {
		r.Use(WithMemSession(RandText(24)))
	}

	//
	// Check default SITE_*
	//
	CheckValue(db, KEY_SITE_LOGO_URL, "/static/img/carrot.svg", ConfigFormatText, true, true)
	CheckValue(db, KEY_SITE_FAVICON_URL, "/static/img/favicon.png", ConfigFormatText, true, true)
	CheckValue(db, KEY_SITE_SIGNIN_URL, "/auth/login", ConfigFormatText, true, true)
	CheckValue(db, KEY_SITE_SIGNUP_URL, "/auth/register", ConfigFormatText, true, true)
	CheckValue(db, KEY_SITE_LOGOUT_URL, "/auth/logout", ConfigFormatText, true, true)
	CheckValue(db, KEY_SITE_RESET_PASSWORD_URL, "/auth/reset_password", ConfigFormatText, true, true)
	CheckValue(db, KEY_SITE_LOGIN_NEXT, "/", ConfigFormatText, true, true)
	CheckValue(db, KEY_SITE_USER_ID_TYPE, "email", ConfigFormatText, true, true)

	InitAuthHandler(GetEnv(ENV_AUTH_PREFIX), db, r)
	return nil
}

func GetCarrotSessionField() string {
	v := GetEnv(ENV_SESSION_FIELD)
	if v == "" {
		return "carrot"
	}
	return v
}
