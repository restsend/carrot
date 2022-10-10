package carrot

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
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
	//SigUserResetpassword: user *User, hash, clientIp, userAgent string
	SigUserResetpassword = "user.resetpassword"
)

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
	return user.Password == HashPassword(password)
}

func SetPassword(db *gorm.DB, user *User, password string) (err error) {
	p := HashPassword(password)
	err = UpdateUserFields(db, user, map[string]interface{}{
		"Password": p,
	})
	if err != nil {
		return
	}
	user.Password = p
	return
}

func HashPassword(password string) string {
	salt := GetEnv(ENV_SALT)
	hashVal := sha256.Sum256([]byte(salt + password))
	return fmt.Sprintf("sha256$%s%x", salt, hashVal)
}

func GetUserByUID(db *gorm.DB, userID uint) (*User, error) {
	var val User
	result := db.Where("id", userID).Where("Enabled", true).Take(&val)
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
		Email:    email,
		Password: HashPassword(password),
		Enabled:  true,
		Actived:  false,
	}

	result := db.Create(&user)
	return &user, result.Error
}

func UpdateUserFields(db *gorm.DB, user *User, vals map[string]interface{}) error {
	return db.Model(user).Updates(vals).Error
}

func SetLastLogin(db *gorm.DB, user *User, lastIp string) error {
	now := time.Now().Truncate(1 * time.Second)
	vals := map[string]interface{}{
		"LastLoginIP": lastIp,
		"LastLogin":   &now,
	}
	user.LastLogin = &now
	user.LastLoginIP = lastIp
	return db.Model(user).Updates(vals).Error
}

func EncodeHashToken(user *User, timestamp int64) (hash string) {
	//
	// ts-uid-token
	logintimestamp := "0"
	if user.LastLogin != nil {
		logintimestamp = fmt.Sprintf("%d", user.LastLogin.Unix())
	}
	t := fmt.Sprintf("%s$%d", user.Email, timestamp)
	salt := GetEnv(ENV_SALT)
	hashVal := sha256.Sum256([]byte(salt + logintimestamp + user.Password + t))
	hash = base64.RawStdEncoding.EncodeToString([]byte(t)) + "-" + fmt.Sprintf("%x", hashVal)
	return hash
}

func DecodeHashToken(db *gorm.DB, hash string) (user *User, err error) {
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
	token := EncodeHashToken(user, ts)
	if token != hash {
		return nil, errors.New("bad token")
	}
	return user, nil
}
