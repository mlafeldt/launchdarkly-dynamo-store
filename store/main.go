package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ld "gopkg.in/launchdarkly/go-client.v3"

	"github.com/mlafeldt/serverless-ldr/dynamodb"
)

func main() {
	lambda.Start(handler)
}

func handler(req *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	// Log some interesting headers
	for _, h := range []string{
		"User-Agent",
		"X-Forwarded-For",
		"X-Amzn-Trace-Id",
		"X-Ld-Signature",
	} {
		log.Printf("DEBUG: %s: %s", h, req.Headers[h])
	}

	// If a webhook secret is provided, verify the signature of the webhook
	// payload to ensure that requests are generated by LaunchDarkly.
	if secret := os.Getenv("LAUNCHDARKLY_WEBHOOK_SECRET"); secret != "" {
		s1 := req.Headers["X-Ld-Signature"]
		s2 := hmacSHA256(req.Body, secret)
		if subtle.ConstantTimeCompare([]byte(s1), []byte(s2)) != 1 {
			log.Printf("ERROR: Invalid webhook payload signature, got %q but want %q", s1, s2)
			return &events.APIGatewayProxyResponse{StatusCode: http.StatusUnauthorized}, nil
		}
		log.Print("INFO: Successfully verified signature of webhook payload")
	} else {
		log.Print("INFO: Skipping signature check of webhook payload")
	}

	// Setting up a LaunchDarkly client with a DynamoDBFeatureStore will
	// sync the data stored in DynamoDB with LaunchDarkly.
	store, err := dynamodb.NewDynamoDBFeatureStore(os.Getenv("LAUNCHDARKLY_DYNAMODB_TABLE"), nil)
	if err != nil {
		log.Printf("ERROR: Failed to initialize DynamoDBFeatureStore: %s", err)
		return &events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	config := ld.DefaultConfig
	config.FeatureStore = store

	ldClient, err := ld.MakeCustomClient(os.Getenv("LAUNCHDARKLY_SDK_KEY"), config, 5*time.Second)
	if err != nil {
		log.Printf("ERROR: Failed to initialize LaunchDarkly client: %s", err)
		return &events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	defer ldClient.Close()

	log.Printf("INFO: Successfully updated the feature store!")

	return &events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}

func hmacSHA256(message string, secret string) string {
	sig := hmac.New(sha256.New, []byte(secret))
	sig.Write([]byte(message))
	return hex.EncodeToString(sig.Sum(nil))
}