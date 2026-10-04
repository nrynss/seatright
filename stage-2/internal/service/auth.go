package service

import (
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf8"

	"github.com/nrynss/keel/id"
	"golang.org/x/crypto/bcrypt"
)

// validEmail reports whether e has the form local@domain: exactly one '@'
// with nonempty local and domain parts and no spaces.
func validEmail(e string) bool {
	if e == "" {
		return false
	}
	for _, c := range e {
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			return false
		}
	}
	at := -1
	for i := 0; i < len(e); i++ {
		if e[i] == '@' {
			if at >= 0 {
				return false
			}
			at = i
		}
	}
	if at <= 0 || at >= len(e)-1 {
		return false
	}
	return true
}

// fieldString extracts an object field that must hold a string. It reports
// the value, whether the field was present, and whether a present field had
// the wrong JSON type (JSON null counts as the wrong type here).
func fieldString(obj map[string]any, name string) (val string, present bool, wrongType bool) {
	raw, ok := obj[name]
	if !ok {
		return "", false, false
	}
	s, ok := raw.(string)
	if !ok {
		return "", true, true
	}
	return s, true, false
}

// Password hashing scheme: SHA-256 prehash, hex-encoded, fed to bcrypt.
// bcrypt rejects inputs longer than 72 bytes, but the specification sets a
// minimum password length of 8 characters and no upper limit, and forbids
// 5xx responses. Hashing the full password with SHA-256 first supports
// arbitrarily long passwords without truncating distinguishing suffixes: two
// passwords differing only past byte 72 still hash differently. The stored
// value keeps the standard bcrypt Modular Crypt Format, so existing hashes
// verify with comparePassword and survive export/import unchanged.
func prehash(password string) []byte {
	sum := sha256.Sum256([]byte(password))
	hexed := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(hexed, sum[:])
	return hexed
}

// hashPassword hashes with bcrypt before the state lock is taken; callers
// re-validate under the lock before using the result.
func hashPassword(password string) (string, error) {
	sum, err := bcrypt.GenerateFromPassword(prehash(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(sum), nil
}

// comparePassword checks a password against a stored hash.
func comparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), prehash(password))
}

// Signup creates an account and its first session token.
func (s *Service) Signup(raw []byte) Result {
	obj, _, err := ParseBody(raw)
	if err != nil {
		return malformed()
	}
	email, present, wrongType := fieldString(obj, "email")
	if wrongType {
		return malformed()
	}
	password, presentPw, wrongTypePw := fieldString(obj, "password")
	if wrongTypePw {
		return malformed()
	}
	display, presentDisplay, wrongTypeDisplay := fieldString(obj, "display_name")
	if wrongTypeDisplay {
		return malformed()
	}
	if !present || !presentPw || !presentDisplay {
		return validationFailed("missing required field")
	}
	if !validEmail(email) {
		return validationFailed("email must have the form local@domain")
	}
	if utf8.RuneCountInString(password) < 8 {
		return validationFailed("password must be at least 8 characters")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return internalErr()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if userByEmail(&s.state, email) != nil {
		return conflict("email_taken", "email already registered")
	}
	userID, err := id.New()
	if err != nil {
		return internalErr()
	}
	token, err := id.New()
	if err != nil {
		return internalErr()
	}
	s.state.Users[userID] = User{ID: userID, Email: email, DisplayName: display, PasswordHash: hash}
	s.state.Tokens[token] = userID
	return created(map[string]any{"user_id": userID, "display_name": display, "token": token})
}

// Login opens a new session token. Tokens never expire and an account may
// hold many concurrent tokens.
func (s *Service) Login(raw []byte) Result {
	obj, _, err := ParseBody(raw)
	if err != nil {
		return malformed()
	}
	email, present, wrongType := fieldString(obj, "email")
	if wrongType {
		return malformed()
	}
	password, presentPw, wrongTypePw := fieldString(obj, "password")
	if wrongTypePw {
		return malformed()
	}
	if !present || !presentPw {
		return validationFailed("missing required field")
	}
	s.mu.Lock()
	u := userByEmail(&s.state, email)
	if u == nil {
		s.mu.Unlock()
		return unauthenticated()
	}
	idCopy, displayCopy, hashCopy := u.ID, u.DisplayName, u.PasswordHash
	s.mu.Unlock()
	if comparePassword(hashCopy, password) != nil {
		return unauthenticated()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cur, ok := s.state.Users[idCopy]
	if !ok || cur.PasswordHash != hashCopy {
		return unauthenticated()
	}
	token, err := id.New()
	if err != nil {
		return internalErr()
	}
	s.state.Tokens[token] = idCopy
	return okResult(map[string]any{"user_id": idCopy, "display_name": displayCopy, "token": token})
}

// Authenticate resolves a bearer token to its owner under the state lock.
func (s *Service) Authenticate(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	uid, ok := s.state.Tokens[token]
	return uid, ok
}
