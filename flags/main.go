package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ld "gopkg.in/launchdarkly/go-client.v3"
)

func main() {
	lambda.Start(handler)
}

func handler(req *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	log.Printf("Webhook payload = %s", req.Body)

	// TODO: verify signature
	log.Printf("Webhook payload signature = %s", req.Headers["X-Ld-Signature"])

	store, err := NewDynamoDBFeatureStore(os.Getenv("DYNAMODB_TABLE"))
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

	log.Print("Successfully updated feature store!")
	return &events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}
