package controllers

import "github.com/Kelhai/ani/services"

var (
	authService services.AuthService
)

func setupServices() {
	authService = services.SetupAuthService()
}
