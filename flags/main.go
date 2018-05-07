package main

import (
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
	for _, h := range []string{
		"User-Agent",
		"X-Forwarded-For",
		"X-Amzn-Trace-Id",
		"X-Ld-Signature", // LaunchDarkly HMAC SHA256 hex digest of webhook payload
	} {
		log.Printf("%s: %s", h, req.Headers[h])
	}

	store, err := dynamodb.NewDynamoDBFeatureStore(os.Getenv("DYNAMODB_TABLE_PREFIX"), nil)
	if err != nil {
		log.Printf("Failed to initialize DynamoDBFeatureStore: %s", err)
		return &events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}

	config := ld.DefaultConfig
	config.FeatureStore = store

	ldClient, err := ld.MakeCustomClient(os.Getenv("LAUNCHDARKLY_SDK_KEY"), config, 5*time.Second)
	if err != nil {
		log.Printf("Failed to initialize LaunchDarkly client: %s", err)
		return &events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	defer ldClient.Close()

	return &events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}
