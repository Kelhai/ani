package controllers

import (
	"github.com/labstack/echo/v5"
)

func SetupAllRoutes(e *echo.Echo) {

	// routes
	setupAuthRoutes(e)
	setupMessageRoutes(e)

	setupServices()
}
