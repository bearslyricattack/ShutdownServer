package main

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
)

// 用于定义JWT的密钥
var secretKey = []byte("your_secret_key")

// Claims 定义 JWT 的结构
type Claims struct {
	DevboxName string `json:"devbox_name"`
	NameSpace  string `json:"namespace"`
	jwt.StandardClaims
}

func main() {
	r := gin.Default()

	// 生成一个简单的 JWT token
	r.GET("/generate-token", func(c *gin.Context) {
		// 创建JWT Token
		expirationTime := time.Now().Add(5 * time.Minute)
		claims := &Claims{
			DevboxName: "devbox01",
			NameSpace:  "default",
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: expirationTime.Unix(),
				Issuer:    "gin-jwt-example",
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(secretKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"token": tokenString})
	})

	// 校验 JWT 并执行操作的接口
	r.GET("/operation", func(c *gin.Context) {
		// 获取请求中的 token
		tokenString := c.DefaultQuery("jwt", "")

		if tokenString == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "JWT token is required"})
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return secretKey, nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		operation := c.DefaultQuery("operation", "")
		if operation == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "operation parameter is required"})
			return
		}
		if operation != "shutdown" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "operation type is error"})
		}

		// 处理 operation

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Operation received: %s", operation),
		})
	})

	r.Run(":8082")
}
