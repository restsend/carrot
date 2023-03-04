package carrot

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const (
	PermissionAll    = "all"
	PermissionCreate = "create"
	PermissionUpdate = "update"
	PermissionRead   = "read"
	PermissionDelete = "delete"
)

const (
	GroupRoleAdmin  = "admin"
	GroupRoleMember = "member"
)

type Config struct {
	ID    uint   `gorm:"primarykey"`
	Key   string `gorm:"size:128;uniqueIndex"`
	Desc  string `gorm:"size:200"`
	Value string
}

type Profile struct {
	Avatar  string `json:"avatar"`
	Gender  string `json:"gender"`
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
	Extra   string `json:"extra"`
}

type User struct {
	ID        uint      `json:"-" gorm:"primarykey"`
	CreatedAt time.Time `json:"-" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"-" gorm:"autoUpdateTime"`

	Email       string     `json:"email" gorm:"size:128;uniqueIndex"`
	Password    string     `json:"-" gorm:"size:128"`
	Phone       string     `json:"phone,omitempty" gorm:"size:64;index"`
	FirstName   string     `json:"firstName,omitempty" gorm:"size:128"`
	LastName    string     `json:"lastName,omitempty" gorm:"size:128"`
	DisplayName string     `json:"displayName,omitempty" gorm:"size:128"`
	IsSuperUser bool       `json:"-"`
	IsStaff     bool       `json:"-"`
	Enabled     bool       `json:"-"`
	Actived     bool       `json:"-"`
	LastLogin   *time.Time `json:"lastLogin,omitempty"`
	LastLoginIP string     `json:"-" gorm:"size:128"`

	Source    string `json:"-" gorm:"size:64;index"`
	Locale    string `json:"locale,omitempty" gorm:"size:20"`
	Timezone  string `json:"timezone,omitempty" gorm:"size:200"`
	Profile   string `json:"profile,omitempty"`
	AuthToken string `json:"token,omitempty" gorm:"-"`
}

type Group struct {
	ID        uint      `json:"-" gorm:"primarykey"`
	CreatedAt time.Time `json:"-" gorm:"autoCreateTime"`
	UpdatedAt time.Time `json:"-"`
	Name      string    `json:"name" gorm:"size:200"`
	Extra     string    `json:"extra"`
}

type GroupMember struct {
	ID      uint   `json:"-" gorm:"primarykey"`
	UserID  uint   `json:"-"`
	User    User   `json:"user"`
	GroupID uint   `json:"-"`
	Group   Group  `json:"group"`
	Role    string `json:"role"`
}

type GroupPermission struct {
	ID      uint   `json:"-" gorm:"primarykey"`
	GroupID uint   `json:"groupId"`
	Group   Group  `json:"-"`
	Content string `json:"content" gorm:"size:200"`
	Code    string `json:"code" gorm:"size:200"`
}

func (u *User) GetVisibleName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.FirstName != "" {
		return u.FirstName
	}
	return u.LastName
}

func (u *User) GetProfile() Profile {
	if u.Profile != "" {
		var val Profile
		err := json.Unmarshal([]byte(u.Profile), &val)
		if err == nil {
			return val
		}
	}
	return Profile{}
}

func InitMigrate(db *gorm.DB) error {
	return MakeMigrates(db, []any{
		&Config{},
		&User{},
		&Group{},
		&GroupMember{},
		&GroupPermission{},
	})
}

// GenUniqueKey generate a unique value for a field in a table.
func GenUniqueKey(tx *gorm.DB, field string, size int) (key string) {
	key = RandText(size)
	for i := 0; i < 10; i++ {
		var c int64
		result := tx.Where(field, key).Limit(1).Count(&c)
		if result.Error != nil {
			break
		}
		if c > 0 {
			continue
		}
		return key
	}
	return ""
}
