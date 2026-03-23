package models

import "time"

type User struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	Email         string     `json:"email"`
	Password_hash string     `json:"-"`
	IsActive      bool       `json:"is_active,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

type UserPublic struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *User) ToPublic() *UserPublic {
	return &UserPublic{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}

func NewUser(name, email, passwordHash string) *User {
	now := time.Now()
	return &User{
		Name:          name,
		Email:         email,
		Password_hash: passwordHash,
		IsActive:      true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Replace defines the logic for a full resource overwrite
func (u *User) PutUpdateFields(newName string, newEmail string) {
	u.Name = newName
	u.Email = newEmail
	// u.UpdatedAt = time.Now()
}

// A mapper that accepts optional values and applies them to the entity
func (u *User) PatchUpdateFields(name *string, email *string) {
	if name != nil {
		u.Name = *name
	}
	if email != nil {
		u.Email = *email
	}
	// u.UpdatedAt = time.Now()
}
