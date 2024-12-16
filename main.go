package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"log"
	"net/http"
	"time"
)

func getK8sClient() (dynamic.Interface, error) {
	kubeconfig := "/etc/kubeconfig/my-kubeconfig"
	fmt.Println("尝试获取kubeconfig", kubeconfig)
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

func shutdownDevbox(devboxName string, namespace string) error {
	log.Println("send message，begin to shutdown server" + devboxName + namespace)
	dynamicClient, err := getK8sClient()
	if err != nil {
		return err
	}

	gvr := schema.GroupVersionResource{
		Group:    "devbox.sealos.io",
		Version:  "v1alpha1",
		Resource: "devboxes",
	}

	resourceClient := dynamicClient.Resource(gvr).Namespace(namespace)

	crdResource, err := resourceClient.Get(context.TODO(), devboxName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get resource: %v", err)
	}

	crdResource, err = resourceClient.Get(context.TODO(), devboxName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get resource: %v", err)
	}

	unstructuredObj := crdResource.DeepCopy()
	unstructuredObj.Object["spec"].(map[string]interface{})["state"] = "Stopped"

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

type req struct {
	Jwt       string `json:"jwt_token"`
	Operation string `json:"operation"`
}

func main() {
	r := gin.Default()
	r.POST("/opsrequest", func(c *gin.Context) {
		fmt.Printf("Request URL: %s\n", c.Request.URL.String()) // 完整 U
		var req req
		if err := c.BindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "JWT token is required"})
			return
		}
		tokenString := req.Jwt
		operation := req.Operation
		fmt.Println(tokenString)
		fmt.Println(operation)
		if tokenString == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "JWT token is required"})
			return
		}

		claims, err := ParseJWT(tokenString)

		if operation == "" {
			log.Println("operation parameter is required")
			c.JSON(http.StatusBadRequest, gin.H{"error": "operation parameter is required"})
			return
		}
		if operation != "shutdown" {
			log.Println("operation type is error")
			c.JSON(http.StatusBadRequest, gin.H{"error": "operation type is error"})
			return
		}

		err = shutdownDevbox(claims.DevboxName, claims.NameSpace)
		if err != nil {
			fmt.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		} else {
			c.JSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("Operation received: %s", operation),
			})
		}
	})

	r.Run(":8082")
}

type DevboxClaims struct {
	DevboxName string `json:"devbox_name"`
	NameSpace  string `json:"namespace"`
	jwt.RegisteredClaims
}

var jwtSecret = []byte("sealos-devbox-shutdown")

// 解析和验证 JWT
func ParseJWT(tokenString string) (*DevboxClaims, error) {
	// 解析和验证 JWT
	token, err := jwt.ParseWithClaims(tokenString, &DevboxClaims{}, func(token *jwt.Token) (interface{}, error) {
		// 验证签名方法是否匹配
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})
	// 检查解析过程是否有误
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}
	// 转换 claims 类型为自定义声明
	claims, ok := token.Claims.(*DevboxClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}
