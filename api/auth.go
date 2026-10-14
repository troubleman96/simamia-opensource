package api

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-sqlite3"
)

const passwordIterations = 600_000
const maxAuthBodyBytes = 16 << 10

type authUser struct {
	Sub         string `json:"sub"`
	Name        string `json:"name"`
	PhoneNumber string `json:"phone_number"`
}

type authRequest struct {
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Password   string `json:"password"`
	RememberMe bool   `json:"remember_me"`
}

func (a *API) authHTTP(w http.ResponseWriter, r *http.Request, requestID string, parts []string) {
	if a.store.db == nil {
		a.fail(w, r, requestID, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "Local accounts require a persistent API database", nil)
		return
	}
	if len(parts) != 1 {
		a.fail(w, r, requestID, http.StatusNotFound, "NOT_FOUND", "Authentication endpoint not found", nil)
		return
	}
	switch parts[0] {
	case "register":
		if r.Method != http.MethodPost {
			a.methodNotAllowed(w, r, requestID, "POST")
			return
		}
		a.registerAccount(w, r, requestID)
	case "login":
		if r.Method != http.MethodPost {
			a.methodNotAllowed(w, r, requestID, "POST")
			return
		}
		a.loginAccount(w, r, requestID)
	case "me":
		if r.Method != http.MethodGet {
			a.methodNotAllowed(w, r, requestID, "GET")
			return
		}
		sub, ok := a.sessionUser(w, r, requestID)
		if !ok {
			return
		}
		user, err := a.loadAuthUser(sub)
		if err != nil {
			a.fail(w, r, requestID, http.StatusUnauthorized, "UNAUTHORIZED", "Local account is no longer available", nil)
			return
		}
		a.write(w, http.StatusOK, Record{"user": user}, requestID, nil)
	case "logout":
		if r.Method != http.MethodPost {
			a.methodNotAllowed(w, r, requestID, "POST")
			return
		}
		if token := bearerToken(r); token != "" {
			_, _ = a.store.db.Exec(`DELETE FROM auth_sessions WHERE token_hash = ?`, hashSessionToken(token))
		}
		a.write(w, http.StatusOK, Record{"logged_out": true}, requestID, nil)
	default:
		a.fail(w, r, requestID, http.StatusNotFound, "NOT_FOUND", "Authentication endpoint not found", nil)
	}
}

func (a *API) registerAccount(w http.ResponseWriter, r *http.Request, requestID string) {
	var input authRequest
	if err := decodeAuthRequest(r, &input); err != nil {
		a.fail(w, r, requestID, http.StatusBadRequest, "INVALID_REQUEST", "Enter valid account details", nil)
		return
	}
	name := strings.TrimSpace(input.Name)
	phone, err := normalizeTanzaniaPhone(input.Phone)
	if len([]rune(name)) < 2 || len([]rune(name)) > 80 || err != nil || len(input.Password) < 8 || len(input.Password) > 72 {
		a.fail(w, r, requestID, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Enter a name, a valid Tanzania phone number, and a password with at least 8 characters", nil)
		return
	}

	passwordHash, err := hashPassword(input.Password)
	if err != nil {
		a.fail(w, r, requestID, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not create local account", nil)
		return
	}
	sub := randomID("usr_")
	_, err = a.store.db.Exec(`INSERT INTO auth_users (sub, full_name, phone, password_hash) VALUES (?, ?, ?, ?)`, sub, name, phone, passwordHash)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code == sqlite3.ErrConstraint {
			a.fail(w, r, requestID, http.StatusConflict, "ACCOUNT_EXISTS", "An account already uses this phone number", nil)
			return
		}
		a.fail(w, r, requestID, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not create local account", nil)
		return
	}
	user := authUser{Sub: sub, Name: name, PhoneNumber: phone}
	a.createAuthSession(w, r, requestID, user, input.RememberMe)
}

func (a *API) loginAccount(w http.ResponseWriter, r *http.Request, requestID string) {
	var input authRequest
	if err := decodeAuthRequest(r, &input); err != nil {
		a.fail(w, r, requestID, http.StatusBadRequest, "INVALID_REQUEST", "Enter a valid phone number and password", nil)
		return
	}
	phone, err := normalizeTanzaniaPhone(input.Phone)
	if err != nil || len(input.Password) == 0 || len(input.Password) > 72 {
		a.fail(w, r, requestID, http.StatusUnauthorized, "UNAUTHORIZED", "Phone number or password is incorrect", nil)
		return
	}
	var user authUser
	var passwordHash string
	err = a.store.db.QueryRow(`SELECT sub, full_name, phone, password_hash FROM auth_users WHERE phone = ?`, phone).Scan(&user.Sub, &user.Name, &user.PhoneNumber, &passwordHash)
	if err != nil || !verifyPassword(passwordHash, input.Password) {
		a.fail(w, r, requestID, http.StatusUnauthorized, "UNAUTHORIZED", "Phone number or password is incorrect", nil)
		return
	}
	a.createAuthSession(w, r, requestID, user, input.RememberMe)
}

func (a *API) createAuthSession(w http.ResponseWriter, r *http.Request, requestID string, user authUser, rememberMe bool) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		a.fail(w, r, requestID, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not start account session", nil)
		return
	}
	token := hex.EncodeToString(tokenBytes)
	duration := 12 * time.Hour
	if rememberMe {
		duration = 30 * 24 * time.Hour
	}
	expiresAt := time.Now().UTC().Add(duration)
	if _, err := a.store.db.Exec(`INSERT INTO auth_sessions (token_hash, user_sub, expires_at) VALUES (?, ?, ?)`, hashSessionToken(token), user.Sub, expiresAt.Format(time.RFC3339Nano)); err != nil {
		a.fail(w, r, requestID, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not start account session", nil)
		return
	}
	a.write(w, http.StatusOK, Record{"access_token": token, "token_type": "Bearer", "expires_at": expiresAt.Format(time.RFC3339), "user": user}, requestID, nil)
}

func (a *API) sessionUser(w http.ResponseWriter, r *http.Request, requestID string) (string, bool) {
	token := bearerToken(r)
	if token == "" {
		a.fail(w, r, requestID, http.StatusUnauthorized, "UNAUTHORIZED", "A local account session is required", nil)
		return "", false
	}
	var sub, expiresAt string
	err := a.store.db.QueryRow(`SELECT user_sub, expires_at FROM auth_sessions WHERE token_hash = ?`, hashSessionToken(token)).Scan(&sub, &expiresAt)
	if err != nil {
		a.fail(w, r, requestID, http.StatusUnauthorized, "UNAUTHORIZED", "Account session is invalid or expired", nil)
		return "", false
	}
	expiry, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil || !expiry.After(time.Now().UTC()) {
		_, _ = a.store.db.Exec(`DELETE FROM auth_sessions WHERE token_hash = ?`, hashSessionToken(token))
		a.fail(w, r, requestID, http.StatusUnauthorized, "UNAUTHORIZED", "Account session is invalid or expired", nil)
		return "", false
	}
	return sub, true
}

func (a *API) loadAuthUser(sub string) (authUser, error) {
	var user authUser
	err := a.store.db.QueryRow(`SELECT sub, full_name, phone FROM auth_users WHERE sub = ?`, sub).Scan(&user.Sub, &user.Name, &user.PhoneNumber)
	return user, err
}

func decodeAuthRequest(r *http.Request, target *authRequest) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxAuthBodyBytes))
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("request must contain one JSON object")
	}
	return nil
}

func normalizeTanzaniaPhone(raw string) (string, error) {
	var digits strings.Builder
	for _, char := range strings.TrimSpace(raw) {
		if unicode.IsDigit(char) && char <= '9' {
			digits.WriteRune(char)
		} else if unicode.IsSpace(char) || strings.ContainsRune("+-().", char) {
			continue
		} else {
			return "", errors.New("invalid phone number")
		}
	}
	value := digits.String()
	value = strings.TrimPrefix(value, "255")
	value = strings.TrimPrefix(value, "0")
	if len(value) != 9 {
		return "", errors.New("phone number must contain 9 local digits")
	}
	return "+255" + value, nil
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derived, err := pbkdf2.Key(sha512.New, password, salt, passwordIterations, 32)
	if err != nil {
		return "", err
	}
	return "pbkdf2-sha512$" + "600000" + "$" + hex.EncodeToString(salt) + "$" + hex.EncodeToString(derived), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2-sha512" || parts[1] != "600000" {
		return false
	}
	salt, saltErr := hex.DecodeString(parts[2])
	expected, hashErr := hex.DecodeString(parts[3])
	if saltErr != nil || hashErr != nil || len(salt) != 16 || len(expected) != 32 {
		return false
	}
	actual, err := pbkdf2.Key(sha512.New, password, salt, passwordIterations, len(expected))
	return err == nil && subtle.ConstantTimeCompare(actual, expected) == 1
}

func hashSessionToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}
