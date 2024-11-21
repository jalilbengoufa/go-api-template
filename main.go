package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-api-template/controllers"
	"github.com/go-api-template/lib/database"
	"github.com/go-api-template/lib/viper"
	customMiddleware "github.com/go-api-template/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"
)

// CustomValidator wraps the validator instance
type CustomValidator struct {
	validator *validator.Validate
}

// Validate is the method that will be used by Echo to validate request structs
func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

func main() {
	db := initializeDatabase()

	e := initializeEcho(db)

	startServer(e)

	waitForShutdown(e)
}

func initializeDatabase() *gorm.DB {
	db, err := database.GetDbInstance()
	if err != nil {
		fmt.Printf("Failed to connect to the database: %v", err)
		closeOnSignal()

	}
	return db
}

func initializeEcho(db *gorm.DB) *echo.Echo {
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Validator = &CustomValidator{validator: validator.New()}
	e.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"message": "Hello, go-api-template API!",
		})
	})

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
		})
	})

	e.POST("/offer/create", func(c echo.Context) error {
		return controllers.TableCreate(c)
	})

	r := e.Group("/restricted")
	{
		r.Use(customMiddleware.EchoEnsureValidToken())
		r.GET("/all", controllers.TableGetAll)
		r.GET("/me/all", controllers.TableGetMeAll)
		r.POST("/table/update", controllers.TableUpdate)
	}

	return e
}

func startServer(e *echo.Echo) {
	appPort := viper.ViperEnvVariable("APP_PORT")
	go func() {
		if err := e.Start(fmt.Sprintf(":%s", appPort)); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Error starting server: %v\n", err)
		}
	}()
}

func waitForShutdown(e *echo.Echo) {
	closeOnSignal()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		fmt.Printf("Error shutting down server: %v\n", err)
	}
	fmt.Println("Shut down gracefully!")
}

func closeOnSignal() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
}
