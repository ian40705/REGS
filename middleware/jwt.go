package middleware

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	if secret := os.Getenv("REGS_JWT_SECRET"); secret != "" {
		JWTSecret = []byte(secret)
		return
	}

	// Fallback to a generated secret when no env var is set.
	// For production, set REGS_JWT_SECRET so the secret is stable across restarts.
	fallback, err := GenerateJWTSecret()
	if err != nil {
		panic(err)
	}
	JWTSecret = []byte(fallback)
}

var (
	//hard coded and should change in production
	JWTSecret = []byte("secret")
	publicKey *ecdsa.PublicKey
)

// GenerateJWTSecret creates a new random 32-byte secret for signing HS256 tokens.
func GenerateJWTSecret() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return hex.EncodeToString(secret), nil
}

// LoadPublicKeyFromFile loads an ECDSA public key (PEM, PKIX) from disk.
func LoadPublicKeyFromFile(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return errors.New("invalid PEM data")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	pk, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("public key is not ECDSA")
	}
	publicKey = pk
	return nil
}

// SignJWT creates a JWT token for login responses.
// The token includes user_id, username, and role claims.
func SignJWT(claims jwt.MapClaims) (string, error) {
	claims["exp"] = jwt.NewNumericDate(time.Now().Add(8 * time.Hour))
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(JWTSecret)
}

// JWTAuth authenticates requests using the Authorization header bearer token.
// It extracts claims and stores role/user_id/username in the Gin context.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		if tokenStr == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		if strings.HasPrefix(strings.ToLower(tokenStr), "bearer ") {
			tokenStr = strings.TrimSpace(tokenStr[7:])
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if publicKey != nil && t.Method.Alg() == jwt.SigningMethodES256.Alg() {
				return publicKey, nil
			}
			return JWTSecret, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if ok {
			c.Set("role", claims["role"])
			c.Set("user_id", claims["user_id"])
			c.Set("username", claims["username"])
		}

		c.Next()
	}
}
