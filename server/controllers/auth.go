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
	g.GET("/user/:userId", getUser, SessionMiddleware)
}

type loginUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token uuid.UUID `json:"token"`
	common.User
}

func login(c *echo.Context) error {
	var bodyUser loginUser

	err := c.Bind(&bodyUser)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Bad json: %s", err.Error()))
	}

	user, err := authService.VerifyPassword(bodyUser.Username, bodyUser.Password)
	if err != nil {
		if errors.Is(err, common.ErrInvalidLogin) {
			return c.NoContent(http.StatusUnauthorized)
		}
		log.Printf("Password verification failed: %s", err.Error())
		return c.NoContent(http.StatusInternalServerError)
	}

	session, err := authService.StartSession(user.Id)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to start session")
	}

	response := loginResponse{
		Token: session.Id,
		User:  *user,
	}

	return c.JSON(http.StatusOK, response)
}

func register(c *echo.Context) error {
	var bodyUser loginUser

	err := c.Bind(&bodyUser)
	if err != nil {
		return c.String(http.StatusBadRequest, fmt.Sprintf("Bad json: %s", err.Error()))
	}

	user, err := authService.CreateUser(bodyUser.Username, bodyUser.Password)
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
	return c.JSON(http.StatusOK, struct{
		Username string `json:"username"`
	}{
		Username: user.Username,
	})
}

