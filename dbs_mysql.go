//go:build mysql

package carrot

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func createDatabaseInstance(cfg *gorm.Config, driver, dsn string) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(dsn), cfg)
}
