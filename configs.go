package carrot

import (
	"os"
	"strconv"
	"strings"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"gorm.io/gorm"
)

var configValueCache *lru.Cache
var configCacheExpired time.Duration = 10 * time.Second

type ConfigCacheItem struct {
	n   time.Time
	val string
}

func init() {
	size := 1024 // fixed size
	v, _ := strconv.ParseInt(GetEnv(ENV_CONFIG_CACHE_SIZE), 10, 32)
	if v > 0 {
		size = int(v)
	}
	configValueCache, _ = lru.New(size)

	exp, err := time.ParseDuration(GetEnv(ENV_CONFIG_CACHE_EXPIRED))
	if err == nil {
		configCacheExpired = exp
	}
}

func GetEnv(key string) string {
	v, _ := LookupEnv(key)
	return v
}

func LookupEnv(key string) (string, bool) {
	// Check .env file
	//
	data, err := os.ReadFile(".env")
	if err != nil {
		return os.LookupEnv(key)

	}
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
		if strings.EqualFold(strings.TrimSpace(vs[0]), key) {
			return strings.TrimSpace(vs[1]), true
		}
	}
	return "", false
}

func SetValue(db *gorm.DB, key, value string) {
	key = strings.ToUpper(key)
	configValueCache.Remove(key)

	var v Config
	result := db.Where("key", key).Take(&v)
	if result.Error != nil {
		newV := &Config{
			Key:   key,
			Value: value,
		}
		db.Create(&newV)
		return
	}
	db.Model(&Config{}).Where("key", key).UpdateColumn("value", value)
}

func GetValue(db *gorm.DB, key string) string {
	key = strings.ToUpper(key)
	cobj, ok := configValueCache.Get(key)
	if ok {
		if time.Since(cobj.(*ConfigCacheItem).n) < configCacheExpired {
			return cobj.(*ConfigCacheItem).val
		}
	}

	var v Config
	result := db.Where("key", key).Take(&v)
	if result.Error != nil {
		return ""
	}

	configValueCache.Add(key, &ConfigCacheItem{
		n:   time.Now(),
		val: v.Value,
	})
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
	v = strings.ToLower(v)
	if v == "1" || v == "yes" || v == "true" {
		return true
	}
	return false
}

func CheckValue(db *gorm.DB, key, defaultValue string) {
	if GetValue(db, key) == "" {
		SetValue(db, key, defaultValue)
	}
}
