package carrot

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var configValueCache *ExpiredLRUCache[string, string]
var envCache *ExpiredLRUCache[string, string]

func init() {
	size := 1024 // fixed size
	v, _ := strconv.ParseInt(GetEnv(ENV_CONFIG_CACHE_SIZE), 10, 32)
	if v > 0 {
		size = int(v)
	}

	var configCacheExpired time.Duration = 10 * time.Second
	exp, err := time.ParseDuration(GetEnv(ENV_CONFIG_CACHE_EXPIRED))
	if err == nil {
		configCacheExpired = exp
	}

	configValueCache = NewExpiredLRUCache[string, string](size, configCacheExpired)
	envCache = NewExpiredLRUCache[string, string](size, configCacheExpired)
}

func GetEnv(key string) string {
	v, _ := LookupEnv(key)
	return v
}

func GetBoolEnv(key string) bool {
	v, _ := strconv.ParseBool(GetEnv(key))
	return v
}

func GetIntEnv(key string) int64 {
	v, _ := strconv.ParseInt(GetEnv(key), 10, 64)
	return v
}

func LookupEnv(key string) (value string, found bool) {
	key = strings.ToUpper(key)
	if envCache != nil {
		if value, found = envCache.Get(key); found {
			return
		}
	}

	defer func() {
		if found && envCache != nil {
			envCache.Add(key, value)
		}
	}()
	// Check .env file
	//
	data, err := os.ReadFile(".env")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for i := 0; i < len(lines); i++ {
			v := strings.TrimSpace(lines[i])
			if v == "" {
				continue
			}
			if v[0] == '#' {
				continue
			}
			if !strings.Contains(v, "=") {
				continue
			}
			vs := strings.SplitN(v, "=", 2)
			k, v := strings.TrimSpace(vs[0]), strings.TrimSpace(vs[1])
			k = strings.ToUpper(k)
			if envCache != nil {
				envCache.Add(k, v)
			}
			if strings.EqualFold(k, key) {
				value = v
				found = true
				break
			}
		}
	}
	if !found {
		value, found = os.LookupEnv(key)
	}
	return
}

// load envs to struct
func LoadEnvs(objPtr any) {
	if objPtr == nil {
		return
	}
	elm := reflect.ValueOf(objPtr).Elem()
	elmType := elm.Type()

	for i := 0; i < elm.NumField(); i++ {
		f := elm.Field(i)
		if !f.CanSet() {
			continue
		}
		keyName := elmType.Field(i).Tag.Get("env")
		if keyName == "-" {
			continue
		}
		if keyName == "" {
			keyName = elmType.Field(i).Name
		}
		switch f.Kind() {
		case reflect.String:
			if v, ok := LookupEnv(keyName); ok {
				f.SetString(v)
			}
		case reflect.Int:
			if v, ok := LookupEnv(keyName); ok {
				if iv, err := strconv.ParseInt(v, 10, 32); err == nil {
					f.SetInt(iv)
				}
			}
		case reflect.Bool:
			if v, ok := LookupEnv(keyName); ok {
				v := strings.ToLower(v)
				if yes, err := strconv.ParseBool(v); err == nil {
					f.SetBool(yes)
				}
			}
		}
	}
}

func SetValue(db *gorm.DB, key, value, format string, autoload, public bool) {
	key = strings.ToUpper(key)
	configValueCache.Remove(key)

	newV := &Config{
		Key:      key,
		Value:    value,
		Format:   format,
		Autoload: autoload,
		Public:   public,
	}
	result := db.Model(&Config{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "format", "autoload", "public"}),
	}).Create(newV)

	if result.Error != nil {
		logrus.WithFields(logrus.Fields{
			"key":    key,
			"value":  value,
			"format": format,
		}).WithError(result.Error).Warn("config: setValue fail")
	}
}

func GetValue(db *gorm.DB, key string) string {
	key = strings.ToUpper(key)
	cobj, ok := configValueCache.Get(key)
	if ok {
		return cobj
	}

	var v Config
	result := db.Where("key", key).Take(&v)
	if result.Error != nil {
		return ""
	}

	configValueCache.Add(key, v.Value)
	return v.Value
}

func GetIntValue(db *gorm.DB, key string, defaultVal int) int {
	v := GetValue(db, key)
	if v == "" {
		return defaultVal
	}
	val, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return defaultVal
	}
	return int(val)
}

func GetBoolValue(db *gorm.DB, key string) bool {
	v := GetValue(db, key)
	if v == "" {
		return false
	}

	r, _ := strconv.ParseBool(strings.ToLower(v))
	return r
}

func CheckValue(db *gorm.DB, key, defaultValue, format string, autoload, public bool) {
	newV := &Config{
		Key:      strings.ToUpper(key),
		Value:    defaultValue,
		Format:   format,
		Autoload: autoload,
		Public:   public,
	}
	db.Model(&Config{}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoNothing: true,
	}).Create(newV)
}

func LoadAutoloads(db *gorm.DB) {
	var configs []Config
	db.Where("autoload", true).Find(&configs)
	for _, v := range configs {
		configValueCache.Add(v.Key, v.Value)
	}
}

func LoadPublicConfigs(db *gorm.DB) []Config {
	var configs []Config
	db.Where("public", true).Find(&configs)
	for _, v := range configs {
		configValueCache.Add(v.Key, v.Value)
	}
	return configs
}
