package controllers

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/go-api-template/lib/database"
	// customMiddleware "github.com/go-api-template/middleware"
	"github.com/google/uuid"
)

// Models and Enums
type Tabler interface {
	TableName() string
}

// TableName overrides the table name used by User to `profiles`
func (TableName) TableName() string {
	return "table_name"
}

type TableName struct {
	ID    uuid.UUID `gorm:"column:id;type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Title *string   `gorm:"column:title;size:255" json:"title"`
}

// Input is the struct for receiving POST request body
type TableInput struct {
	Title *string `json:"title"`
}

func TableGetMeAll(c echo.Context) error {
	db, err := database.GetDbInstance()
	if err != nil {
		fmt.Print("Failed to get GetDbInstance OfferGetMeAll")
	}
	result := []TableName{}

	// claims := customMiddleware.ParseClaims(c)

	if err := db.Find(&result).Error; err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not TableName found"})
	}
	return c.JSON(http.StatusOK, result)
}

func TableGetAll(c echo.Context) error {
	db, err := database.GetDbInstance()
	if err != nil {
		fmt.Print("Failed to get GetDbInstance OfferGetMeAll")
	}
	result := []TableName{}
	if err := db.Find(&result).Error; err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "not table found"})
	}

	return c.JSON(http.StatusOK, result)
}

func TableCreate(c echo.Context) error {
	db, err := database.GetDbInstance()
	if err != nil {
		fmt.Print("Failed to get GetDbInstance OfferCreate")
	}

	var input TableInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid input"})
	}

	if err := c.Validate(&input); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error while validating table input": err.Error()})
	}

	// Create
	tableInput := TableName{
		ID:    uuid.New(),
		Title: input.Title,
	}

	if err := db.Create(&tableInput).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to create TableName"})
	}

	return c.JSON(http.StatusOK, echo.Map{"data": tableInput})
}

type TableUpdateInput struct {
	Title *string `json:"title"`
	ID    *string `json:"id"`
}

func TableUpdate(c echo.Context) error {
	db, err := database.GetDbInstance()
	if err != nil {
		fmt.Print("Failed to get GetDbInstance OfferUpdate")
	}

	var input TableUpdateInput
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "Invalid input"})
	}

	if err := c.Validate(&input); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error while validating offer input": err.Error()})
	}

	if err := db.Model(&TableName{}).Where("id = ?", input.ID).Update("title", input.Title).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "Failed to update TableName"})
	}

	return c.JSON(http.StatusOK, echo.Map{"data": input.ID})
}
