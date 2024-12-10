package main

import (
	"context"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"log"
	"net/http"
	"path/filepath"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// 用于定义JWT的密钥
var secretKey = []byte("your_secret_key")

// Claims 定义 JWT 的结构
type Claims struct {
	DevboxName string `json:"devbox_name"`
	NameSpace  string `json:"namespace"`
	jwt.StandardClaims
}

func getK8sClient() (dynamic.Interface, error) {
	kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("could not get kubeconfig: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create dynamic client: %v", err)
	}

	return dynamicClient, nil
}

func updateCRDResource() error {
	dynamicClient, err := getK8sClient()
	if err != nil {
		return err
	}

	// 获取 CRD 资源
	// 使用 schema.GroupVersionResource 替换 metav1.GroupVersionResource

	// 定义 GroupVersionResource

	gvr := schema.GroupVersionResource{
		Group:    "example.com", // CRD 的 API 组
		Version:  "v1",          // CRD 的版本
		Resource: "myresources", // CRD 的资源名称
	}

	// 使用 dynamicClient 获取 CRD 资源的客户端
	resourceClient := dynamicClient.Resource(gvr).Namespace("default")

	// 获取 CRD 资源
	crdResource, err := resourceClient.Get(context.TODO(), "myresource-name", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get resource: %v", err)
	}

	crdResource, err = resourceClient.Get(context.TODO(), "myresource-name", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get resource: %v", err)
	}

	// 假设我们要修改 spec.someField 字段
	// 修改 CRD 资源的某个字段
	unstructuredObj := crdResource.DeepCopy() // 克隆资源对象
	unstructuredObj.Object["spec"].(map[string]interface{})["someField"] = "new value"

	// 使用重试机制进行更新
	err = retry.OnError(wait.Backoff{
		Duration: 1 * time.Second,
		Factor:   2.0,
		Steps:    5,
	}, func(error) bool { return true }, func() error {
		_, err := resourceClient.Update(context.TODO(), unstructuredObj, metav1.UpdateOptions{})
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to update resource: %v", err)
	}

	log.Println("Successfully updated the CRD resource!")
	return nil
}

// 自定义的重试机制
func retryOnError(retries int, delay time.Duration, fn func() error) error {
	for i := 0; i < retries; i++ {
		err := fn()
		if err == nil {
			return nil
		}
		log.Printf("Attempt %d failed: %v", i+1, err)
		time.Sleep(delay)
	}
	return fmt.Errorf("all attempts failed")
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
