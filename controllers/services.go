package controllers

import "github.com/Kelhai/ani/services"

var (
	authService services.AuthService
	messageService services.MessageService
)

func setupServices() {
	authService = services.SetupAuthService()
	messageService = services.SetupMessageService()
}
