package carrot

import (
	"io"
	"log"
	"os"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDatabase(logWrite io.Writer, driver, dsn string) (*gorm.DB, error) {
	if driver == "" {
		driver = GetEnv(ENV_DB_DRIVER)
	}
	if dsn == "" {
		dsn = GetEnv(ENV_DSN)
	}

	var newLogger logger.Interface
	if logWrite == nil {
		logWrite = os.Stdout
	}

	newLogger = logger.New(
		log.New(logWrite, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  false,       // Disable color
		},
	)

	cfg := &gorm.Config{
		Logger:                 newLogger,
		SkipDefaultTransaction: true,
	}

	return createDatabaseInstance(cfg, driver, dsn)
}

func MakeMigrates(db *gorm.DB, insts []any) error {
	for _, v := range insts {
		if err := db.AutoMigrate(v); err != nil {
			return err
		}
	}
	return nil
}
