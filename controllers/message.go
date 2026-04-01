package controllers

import (
	"github.com/labstack/echo/v5"
)

func setupMessageRoutes(e *echo.Echo) {
	g := e.Group("/messages")
	g.Use(SessionMiddleware)

	g.POST("/:conversationId", sendMessage)
}

func sendMessage(c *echo.Context) error {
	return nil
}

