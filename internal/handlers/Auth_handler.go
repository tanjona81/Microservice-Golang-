package handlers

import (
	"errors"
	"example/hello/internal/config"
	"example/hello/internal/domain"
	"example/hello/internal/dto"
	"example/hello/internal/services"
	"example/hello/internal/utils"
	"log/slog"
	"net/http"
	"time"
)

type AuthHandler struct {
	appConfig    *config.Config
	authService  services.AuthService
	tokenService services.TokenService
}

func NewAuthHandler(cfg *config.Config, a services.AuthService, t services.TokenService) *AuthHandler {
	return &AuthHandler{
		appConfig:    cfg,
		authService:  a,
		tokenService: t,
	}
}

func (handle *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	isSecure := handle.appConfig.AppEnv.IsSecure()

	cookieName := "refresh_token"
	if isSecure {
		cookieName = "__Host-refresh_token"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,              // Instant deletion
		Expires:  time.Unix(0, 0), // Old-school fail-safe for older browsers
	})
}

func (handle *AuthHandler) setRefreshTokenCookieHightestSecurity(w http.ResponseWriter, token string) {
	ttl := handle.appConfig.Security.RefreshTokenTTL
	isSecure := handle.appConfig.AppEnv.IsSecure()

	cookieName := "refresh_token"
	if isSecure {
		cookieName = "__Host-refresh_token"
	}

	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(ttl.Seconds()),
	}
	http.SetCookie(w, cookie)
}

func (h *AuthHandler) setRefreshTokenCookie(w http.ResponseWriter, token string) {
	ttl := h.appConfig.Security.RefreshTokenTTL

	cookie := &http.Cookie{
		Name:     "__Secure-refresh_token",
		Value:    token,
		Path:     "/api/v1/auth/refresh",
		HttpOnly: true,
		Secure:   h.appConfig.AppEnv.IsSecure(),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(ttl.Seconds()),
	}
	http.SetCookie(w, cookie)
}

func (handle *AuthHandler) execFullSessionRevocation(w http.ResponseWriter, r *http.Request, token string) {
	// Wipe browser state immediately
	handle.clearRefreshCookie(w)

	// Wipe DB state (Security)
	if token != "" {
		// Invalidate the session in the database
		// We don't want to stop the logout if the DB fails, but we should log it
		err := handle.tokenService.RevokeSession(r.Context(), token)
		if err != nil {
			// Log this error internally
			slog.Warn("[WARNING] Failed to revoke session in database during logout",
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
				"trace_id", r.Header.Get("X-Request-ID"),
			)
		}
	}
}

// login
func (handle *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// decoding logic
	req, err := utils.JSONDecoder[dto.LoginRequest](w, r)
	if err != nil {
		return
	}

	// Get Ip address
	ip := utils.GetClientIP(r)

	// The service now returns the User object and the Roles slice
	resLogin, err := handle.authService.Login(r.Context(), req.Email, req.Password, ip)
	if err != nil {
		utils.HandleError(w, r, err)
		slog.Debug("Auth Handler error", "Error", err)
		return
	}

	// slog.Debug("Auth hadler Login", "refresh token", string(resLogin.RefreshToken))

	handle.setRefreshTokenCookieHightestSecurity(w, resLogin.RefreshToken)

	// Wrap it in our Senior DTO
	resp := &dto.LoginResponse{
		AccessToken: resLogin.AccessToken,
		User: &dto.UserStats{
			Name:  resLogin.User.Name,
			Email: resLogin.User.Email,
			Roles: resLogin.Roles,
		},
	}

	utils.SendSuccessWithMetadata(w, http.StatusOK, resp, map[string]string{"ExpiresAt": resLogin.Expiry.Format(time.RFC3339)})
}

// logout
func (handle *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	// Decode JSON
	req, err := utils.JSONDecoder[dto.CreateUserRequest](w, r)
	if err != nil {
		utils.HandleError(w, r, err)
		return
	}

	// Invalidate the session in the database
	// We don't want to stop the logout if the DB fails, but we should log it
	registerRes, err := handle.authService.Register(r.Context(), req)
	if err != nil {
		utils.HandleError(w, r, err)
		return
	}

	handle.setRefreshTokenCookieHightestSecurity(w, registerRes.RefreshToken)

	// Wrap it in our DTO
	resp := &dto.LoginResponse{
		AccessToken: registerRes.AccessToken,
		User: &dto.UserStats{
			Name:  registerRes.User.Name,
			Email: registerRes.User.Email,
			Roles: registerRes.Roles,
		},
	}

	utils.SendSuccessWithMetadata(w, http.StatusOK, resp, map[string]string{"ExpiresAt": registerRes.Expiry.Format(time.RFC3339)})
}

// refresh token
func (handle *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	// Get the Refresh Token from the secure cookie
	cookieName := "refresh_token"
	if handle.appConfig.AppEnv.IsSecure() {
		cookieName = "__Host-refresh_token"
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		// No cookie means the user is not logged in or session expired
		utils.HandleError(w, r, domain.NewUnauthorizedError(errors.New("refresh token missing")))
		return
	}

	// Call the service to rotate the session
	// This will: Validate the old token, delete it, create a new one, and return user info
	result, errSession := handle.tokenService.RotateSession(r.Context(), cookie.Value)
	if errSession != nil {
		// If the token was stolen or reused, the service returns an error
		var appErr *domain.AppError

		// Check if it's an AppError AND has a 401 code
		// OR check if it's the specific Compromised error
		isUnauthorized := errors.As(errSession, &appErr) && appErr.Code == 401
		isCompromised := errors.Is(errSession, domain.ErrCompromisedSession)

		if isUnauthorized || isCompromised {
			handle.execFullSessionRevocation(w, r, cookie.Value)
		}
		utils.HandleError(w, r, errSession)
		return
	}

	handle.setRefreshTokenCookieHightestSecurity(w, result.RefreshToken)

	// Return the new Access Token to the frontend
	resp := &dto.LoginResponse{
		AccessToken: result.AccessToken,
		User: &dto.UserStats{
			Name:  result.User.Name,
			Email: result.User.Email,
			Roles: result.Roles,
		},
	}

	utils.SendSuccessWithMetadata(w, http.StatusOK, resp, map[string]string{
		"ExpiresAt": result.Expiry.Format(time.RFC3339),
	})
}

// logout
func (handle *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Retrieve the cookie
	cookieName := "refresh_token"
	if handle.appConfig.AppEnv.IsSecure() {
		cookieName = "__Host-refresh_token"
	}
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		// If there is no cookie, they are already "logged out"
		utils.SendSuccess(w, http.StatusOK, "Already logged out")
		return
	}

	// Clear the cookie in the browser
	handle.execFullSessionRevocation(w, r, cookie.Value)

	utils.SendSuccess(w, http.StatusOK, "Logged out successfully")
}
