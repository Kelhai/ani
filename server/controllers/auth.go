package controllers

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/Kelhai/ani/common"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func setupAuthRoutes(e *echo.Echo) {
	g := e.Group("/auth")

	g.POST("/login", login)
	g.POST("/register", register)
	g.GET("/user/by-id/:userId", getUser, SessionMiddleware)
	g.GET("/user/:username", getUserByUsername, SessionMiddleware)
}

func login(c *echo.Context) error {
	var envelope common.AuthEnvelope
	if err := c.Bind(&envelope); err != nil {
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
	if err := c.Bind(&bodyUser); err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Bad json: %s", err.Error()))
	}

	user, err := authService.CreateUser(bodyUser.Username, bodyUser.IdentityPk, bodyUser.KemPk, bodyUser.KemPkSignature)
	if err != nil {
		return c.String(http.StatusConflict, "User already exists")
	}

	return c.JSON(http.StatusCreated, user)
}

func getUser(c *echo.Context) error {
	userId, err := uuid.Parse(c.Param("userId"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	user, err := authService.GetUserById(userId)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}
	return c.JSON(http.StatusOK, struct {
		Username string `json:"username"`
	}{Username: user.Username})
}

// getUserByUsername returns the full public key bundle for a username. This is
// what clients call to initiate a session with someone.
func getUserByUsername(c *echo.Context) error {
	username := c.Param("username")
	if username == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing username")
	}

	user, err := authService.GetUserByUsername(username)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	return c.JSON(http.StatusOK, common.User{
		Id:             user.Id,
		Username:       user.Username,
		IdentityPk:     user.IdentityPk,
		KemPk:          user.KemPk,
		KemPkSignature: user.KemPkSignature,
	})
}
