package dto

import (
	"example/hello/internal/models"
)

type PatchUserRequest struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

type PutUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type PaginationMetadata struct {
	CurrentPage  int `json:"current_page"`
	PageSize     int `json:"page_size"`
	TotalRecords int `json:"total_records"`
	TotalPages   int `json:"total_pages"`
}

type CursorPaginationMetadata struct {
	NextCursor   string `json:"next_cursor"`
	HasMore      bool   `json:"has_more"`
	TotalRecords int    `json:"total_records"`
	TotalPages   int    `json:"total_pages"`
}

type UserListResponse struct {
	Metadata PaginationMetadata `json:"metadata"`
	Data     []models.User      `json:"data"`
}

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=2,max=250"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type UserStats struct {
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

type LoginResponse struct {
	AccessToken string     `json:"access_token"`
	User        *UserStats `json:"user"`
}
