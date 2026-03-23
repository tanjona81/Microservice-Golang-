package dto

import (
	"example/hello/internal/models"
	"time"
)

type TokenRotationResult struct {
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	Expiry       time.Time          `json:"expiry"`
	User         *models.UserPublic `json:"user"`
	Roles        []string           `json:"roles"`
}

type LoginResult struct {
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	Expiry       time.Time          `json:"expiry"`
	User         *models.UserPublic `json:"user"`
	Roles        []string           `json:"roles"`
}

type RegisterResult struct {
	User         *models.User `json:"user"`
	Roles        []string     `json:"roles"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	Expiry       time.Time    `json:"expiry"`
}
