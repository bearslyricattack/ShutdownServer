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

var secretKey = []byte("your_secret_key")

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
	Jwt       string `json:"jwt"`
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
		claims := &Claims{}
		_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return secretKey, nil
		})
		fmt.Println(claims)
		if err != nil {
			log.Println(err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if operation == "" {
			log.Println("operation parameter is required")
			c.JSON(http.StatusBadRequest, gin.H{"error": "operation parameter is required"})
			return
		}
		if operation != "shutdown" {
			log.Println("operation type is error")
			c.JSON(http.StatusBadRequest, gin.H{"error": "operation type is error"})
		}

		err = shutdownDevbox(claims.DevboxName, claims.NameSpace)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Operation received: %s", operation),
		})
	})

	r.Run(":8082")
}
