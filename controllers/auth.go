package controllers

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v5"
)

func setupAuthRoutes(e *echo.Echo) {
	g := e.Group("/auth")

	g.POST("/login", login)
}

type loginUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func login(c *echo.Context) error {
	var bodyUser loginUser

	err := c.Bind(&bodyUser)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Bad json: %s", err.Error()))
	}

	user, err := authService.Login(bodyUser.Username, bodyUser.Password)

	return nil
}
