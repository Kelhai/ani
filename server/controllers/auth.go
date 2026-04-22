package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/Kelhai/ani/common"
	"github.com/labstack/echo/v5"
)

func setupAuthRoutes(e *echo.Echo) {
	g := e.Group("/auth")

	g.POST("/login", login)
	g.POST("/register", register)
	g.GET("/user/:username", getUser, SessionMiddleware)
}

func login(c *echo.Context) error {
	var envelope common.AuthEnvelope
	err := c.Bind(&envelope)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Bad json: %s", err.Error()))
	}

	user, err := authService.VerifyEnvelope(envelope)
	if err != nil {
		if errors.Is(err, common.ErrInvalidLogin) {
			return c.NoContent(http.StatusUnauthorized)
		}
		log.Printf("Signed login failed: %s", err.Error())
		return c.NoContent(http.StatusInternalServerError)
	}

	session, err := authService.StartSession(user.Id)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to start session")
	}

	return c.JSON(http.StatusOK, common.Session{
		Id:        session.Id,
		UserId:    user.Id,
		ExpiresAt: session.ExpiresAt,
	})
}

func register(c *echo.Context) error {
	var bodyUser common.RegisterRequest

	err := c.Bind(&bodyUser)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Bad json: %s", err.Error()))
	}

	user, err := authService.CreateUser(bodyUser.Username, bodyUser.IdentityPk, bodyUser.KemPk, bodyUser.KemPkSignature)
	if err != nil {
		return c.String(http.StatusConflict, "User already exists")
	}

	return c.JSON(http.StatusCreated, user)
}

func getUser(c *echo.Context) error {
	username := c.Param("username")
	if len(username) < 4 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid username")
	}

	user, err := authService.GetUserByUsername(username)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return c.JSON(http.StatusOK, user)
}
