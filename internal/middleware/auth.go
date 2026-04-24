package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"api-gateway/internal/config"
	"api-gateway/internal/domain"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuth(jwtCfg config.JWTConfig) gin.HandlerFunc {
	if !jwtCfg.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	parser := jwt.NewParser(jwt.WithLeeway(30 * time.Second))

	return func(c *gin.Context) {
		tokenStr, err := bearerToken(c.GetHeader("Authorization"))
		if err != nil {
			domain.WriteError(c, http.StatusUnauthorized, "AUTH_MISSING_TOKEN", "Missing or invalid Authorization header")
			c.Abort()
			return
		}

		token, err := parser.Parse(tokenStr, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return []byte(jwtCfg.HMACSecret), nil
		})
		if err != nil || !token.Valid {
			domain.WriteError(c, http.StatusUnauthorized, "AUTH_INVALID_TOKEN", "Token validation failed")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			domain.WriteError(c, http.StatusUnauthorized, "AUTH_INVALID_CLAIMS", "Token claims are invalid")
			c.Abort()
			return
		}

		if jwtCfg.Issuer != "" && claims["iss"] != jwtCfg.Issuer {
			domain.WriteError(c, http.StatusUnauthorized, "AUTH_INVALID_ISSUER", "Token issuer is not allowed")
			c.Abort()
			return
		}
		if jwtCfg.Audience != "" && !matchAudience(claims["aud"], jwtCfg.Audience) {
			domain.WriteError(c, http.StatusUnauthorized, "AUTH_INVALID_AUDIENCE", "Token audience is not allowed")
			c.Abort()
			return
		}

		c.Set("jwt_claims", claims)
		c.Next()
	}
}

func bearerToken(authHeader string) (string, error) {
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("invalid authorization header")
	}
	return parts[1], nil
}

func matchAudience(claimValue any, required string) bool {
	switch v := claimValue.(type) {
	case string:
		return v == required
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == required {
				return true
			}
		}
	}
	return false
}
