package viper

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func ViperEnvVariable(key string) string {
	if os.Getenv("APP_ENV") != "PRODUCTION" {
		err := godotenv.Load(".env")
		if err != nil {
			log.Println(".env file not found, relying on environment variables")
		}

	}

	viper.AutomaticEnv()

	value := viper.GetString(key)

	return value
}
