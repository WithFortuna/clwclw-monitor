package httpapi

import (
	"crypto/rand"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const jwtTokenExpiry = 24 * time.Hour

var jwtSigningKey []byte

func initJWTKey(secret string) {
	if secret != "" {
		jwtSigningKey = []byte(secret)
		return
	}
	// Generate a random key if no secret is configured.
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate JWT key: " + err.Error())
	}
	jwtSigningKey = b
}

func generateJWT(userID, username string) (string, error) {
	claims := jwt.MapClaims{
		"sub":      userID,
		"username": username,
		"exp":      time.Now().Add(jwtTokenExpiry).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSigningKey)
}

func generateJWTFromClaims(claimsMap map[string]any) (string, error) {
	claims := jwt.MapClaims{}
	for k, v := range claimsMap {
		claims[k] = v
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSigningKey)
}

func parseJWT(tokenStr string) (userID string, username string, err error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSigningKey, nil
	})
	if err != nil {
		return "", "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", jwt.ErrSignatureInvalid
	}
	sub, _ := claims["sub"].(string)
	uname, _ := claims["username"].(string)
	return sub, uname, nil
}
