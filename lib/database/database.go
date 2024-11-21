package database

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-api-template/lib/viper"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	dbInstance *gorm.DB
	once       sync.Once
)

func GetDbInstance() (*gorm.DB, error) {
	host := viper.ViperEnvVariable("PG_HOST")
	user := viper.ViperEnvVariable("PG_USER")
	dbName := viper.ViperEnvVariable("PG_DB")
	password := viper.ViperEnvVariable("PG_PASSWORD")
	port := viper.ViperEnvVariable("PG_PORT")

	var initErr error

	once.Do(func() {
		dsn := fmt.Sprintf("host=%s user=%s dbname=%s password=%s port=%s", host, user, dbName, password, port)
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			initErr = err
			return
		}

		sqlDB, err := db.DB()
		if err != nil {
			initErr = err
			return
		}

		sqlDB.SetMaxOpenConns(25)
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetConnMaxLifetime(time.Minute)

		dbInstance = db
	})

	if initErr != nil {
		return nil, initErr
	}
	return dbInstance, nil
}
