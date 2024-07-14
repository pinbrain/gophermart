package utils

import (
	"errors"
	"fmt"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v4"
	"github.com/pinbrain/gophermart/internal/model"
	"golang.org/x/crypto/bcrypt"
)

const (
	jwtExpires   = time.Hour * 3
	jwtSecretKey = "some_secret_jwt_key"
)

type JWTClaims struct {
	jwt.RegisteredClaims
	UserID int
	Login  string
}

func GeneratePasswordHash(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to generate pwd hash: %w", err)
	}
	return string(hashedBytes), nil
}

func ComparePwdAndHash(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func BuildJWTSting(user model.User) (string, error) {
	if user.ID == 0 || user.Login == "" {
		return "", errors.New("not valid user data")
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, JWTClaims{
		UserID: user.ID,
		Login:  user.Login,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtExpires)),
		},
	})

	tokenString, err := token.SignedString([]byte(jwtSecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to build jwt string: %w", err)
	}
	return tokenString, nil
}

func GetJWTClaims(tokenString string) (*JWTClaims, error) {
	claims := &JWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(jwtSecretKey), nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse jwt token: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("invalid jwt token")
	}
	return claims, nil
}

func IsValidOrderNum(orderNumber string) bool {
	var sum int
	double := false

	for i := len(orderNumber) - 1; i >= 0; i-- {
		r := rune(orderNumber[i])
		if !unicode.IsDigit(r) {
			return false
		}

		digit := int(r - '0')
		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		double = !double
	}

	return sum%10 == 0
}
