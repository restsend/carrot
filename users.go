package carrot

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	//SigUserLogin: user *User, c *gin.Context
	SigUserLogin = "user.login"
	//SigUserLogout: user *User, c *gin.Context
	SigUserLogout = "user.logout"
	//SigUserCreate: user *User, c *gin.Context
	SigUserCreate = "user.create"
	//SigUserVerifyEmail: user *User, hash, clientIp, userAgent string
	SigUserVerifyEmail = "user.verifyemail"
	//SigUserResetPassword: user *User, hash, clientIp, userAgent string
	SigUserResetPassword = "user.resetpassword"
)

func InTimezone(c *gin.Context, timezone string) {
	tz, err := time.LoadLocation(timezone)
	if err != nil {
		return
	}
	c.Set(TzField, tz)

	session := sessions.Default(c)
	session.Set(TzField, timezone)
	session.Save()
}

func CurrentTimezone(c *gin.Context) *time.Location {
	if cachedObj, exists := c.Get(TzField); exists && cachedObj != nil {
		return cachedObj.(*time.Location)
	}

	session := sessions.Default(c)
	tzkey := session.Get(TzField)

	if tzkey == nil {
		if user := CurrentUser(c); user != nil {
			tzkey = user.Timezone
		}
	}

	var tz *time.Location
	defer func() {
		if tz == nil {
			tz = time.UTC
		}
		c.Set(TzField, tz)
	}()

	if tzkey == nil {
		return time.UTC
	}

	tz, _ = time.LoadLocation(tzkey.(string))
	if tz == nil {
		return time.UTC
	}
	return tz
}

func AuthRequired(c *gin.Context) {
	if CurrentUser(c) != nil {
		c.Next()
		return
	}

	token := c.GetHeader("Authorization")
	if token == "" {
		token = c.Query("token")
	}

	if token == "" {
		AbortWithJSONError(c, http.StatusUnauthorized, errors.New("authorization required"))
		return
	}

	db := c.MustGet(DbField).(*gorm.DB)
	// split bearer
	token = strings.TrimPrefix(token, "Bearer ")
	user, err := DecodeHashToken(db, token, false)
	if err != nil {
		AbortWithJSONError(c, http.StatusUnauthorized, err)
		return
	}
	c.Set(UserField, user)
	c.Next()
}

func CurrentUser(c *gin.Context) *User {
	if cachedObj, exists := c.Get(UserField); exists && cachedObj != nil {
		return cachedObj.(*User)
	}

	session := sessions.Default(c)
	userId := session.Get(UserField)
	if userId == nil {
		return nil
	}

	db := c.MustGet(DbField).(*gorm.DB)
	user, err := GetUserByUID(db, userId.(uint))
	if err != nil {
		return nil
	}
	c.Set(UserField, user)
	return user
}

func CurrentGroup(c *gin.Context) *Group {
	if cachedObj, exists := c.Get(GroupField); exists && cachedObj != nil {
		return cachedObj.(*Group)
	}

	session := sessions.Default(c)
	groupId := session.Get(GroupField)
	if groupId == nil {
		return nil
	}

	db := c.MustGet(DbField).(*gorm.DB)
	group, err := GetGroupByID(db, groupId.(uint))
	if err != nil {
		return nil
	}
	c.Set(GroupField, group)
	return group
}

func SwitchGroup(c *gin.Context, group *Group) {
	session := sessions.Default(c)
	session.Set(GroupField, group.ID)
	session.Save()
}

func Login(c *gin.Context, user *User) {
	db := c.MustGet(DbField).(*gorm.DB)
	SetLastLogin(db, user, c.ClientIP())
	session := sessions.Default(c)
	session.Set(UserField, user.ID)
	session.Save()
	Sig().Emit(SigUserLogin, user, c)
}

func Logout(c *gin.Context, user *User) {
	c.Set(UserField, nil)
	session := sessions.Default(c)
	session.Delete(UserField)
	session.Save()
	Sig().Emit(SigUserLogout, user, c)
}

func CheckPassword(user *User, password string) bool {
	if user.Password == "" {
		return false
	}
	return user.Password == HashPassword(password)
}

func SetPassword(db *gorm.DB, user *User, password string) (err error) {
	p := HashPassword(password)
	err = UpdateUserFields(db, user, map[string]any{
		"Password": p,
	})
	if err != nil {
		return
	}
	user.Password = p
	return
}

func HashPassword(password string) string {
	if password == "" {
		return ""
	}
	salt := GetEnv(ENV_SALT)
	hashVal := sha256.Sum256([]byte(salt + password))
	return fmt.Sprintf("sha256$%x", hashVal)
}

func GetUserByUID(db *gorm.DB, userID uint) (*User, error) {
	var val User
	result := db.Where("id", userID).Where("enabled", true).Take(&val)
	if result.Error != nil {
		return nil, result.Error
	}
	return &val, nil
}

func GetUserByEmail(db *gorm.DB, email string) (user *User, err error) {
	var val User
	result := db.Where("email", strings.ToLower(email)).Take(&val)
	if result.Error != nil {
		return nil, result.Error
	}
	return &val, nil
}

func IsExistsByEmail(db *gorm.DB, email string) bool {
	_, err := GetUserByEmail(db, email)
	return err == nil
}

func CreateUser(db *gorm.DB, email, password string) (*User, error) {
	user := User{
		Email:     email,
		Password:  HashPassword(password),
		Enabled:   true,
		Activated: false,
	}

	result := db.Create(&user)
	return &user, result.Error
}

func DeactiveUser(db *gorm.DB, user *User) error {
	Warning("DeactiveUser", user.ID, user.Email)
	return db.Delete(user).Error
}

func UpdateUserFields(db *gorm.DB, user *User, vals map[string]any) error {
	return db.Model(user).Updates(vals).Error
}

func SetLastLogin(db *gorm.DB, user *User, lastIp string) error {
	now := time.Now().Truncate(1 * time.Second)
	vals := map[string]any{
		"LastLoginIP": lastIp,
		"LastLogin":   &now,
	}
	user.LastLogin = &now
	user.LastLoginIP = lastIp
	return db.Model(user).Updates(vals).Error
}

func EncodeHashToken(user *User, timestamp int64, useLastlogin bool) (hash string) {
	//
	// ts-uid-token
	logintimestamp := "0"
	if useLastlogin && user.LastLogin != nil {
		logintimestamp = fmt.Sprintf("%d", user.LastLogin.Unix())
	}
	t := fmt.Sprintf("%s$%d", user.Email, timestamp)
	salt := GetEnv(ENV_SALT)
	hashVal := sha256.Sum256([]byte(salt + logintimestamp + user.Password + t))
	hash = base64.RawStdEncoding.EncodeToString([]byte(t)) + "-" + fmt.Sprintf("%x", hashVal)
	return hash
}

func DecodeHashToken(db *gorm.DB, hash string, useLastLogin bool) (user *User, err error) {
	vals := strings.Split(hash, "-")
	if len(vals) != 2 {
		return nil, errors.New("bad token")
	}
	data, err := base64.RawStdEncoding.DecodeString(vals[0])
	if err != nil {
		return nil, errors.New("bad token")
	}

	vals = strings.Split(string(data), "$")
	if len(vals) != 2 {
		return nil, errors.New("bad token")
	}

	ts, err := strconv.ParseInt(vals[1], 10, 64)
	if err != nil {
		return nil, errors.New("bad token")
	}

	if time.Now().Unix() > ts {
		return nil, errors.New("token expired")
	}

	user, err = GetUserByEmail(db, vals[0])
	if err != nil {
		return nil, errors.New("bad token")
	}
	token := EncodeHashToken(user, ts, useLastLogin)
	if token != hash {
		return nil, errors.New("bad token")
	}
	return user, nil
}

func CheckUserAllowLogin(db *gorm.DB, user *User) error {
	if !user.Enabled {
		return errors.New("user not allow login")
	}

	if GetBoolValue(db, KEY_USER_ACTIVATED) && !user.Activated {
		return errors.New("waiting for activation")
	}
	return nil
}

// Build a token for user.
// If useLoginTime is true, the token will be expired after user login.
func BuildAuthToken(db *gorm.DB, user *User, expired time.Duration, useLoginTime bool) string {
	n := time.Now().Add(expired)
	return EncodeHashToken(user, n.Unix(), useLoginTime)
}
