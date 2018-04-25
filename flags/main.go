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
	log.Printf("Received webhook payload: %s", req.Body)

	// TODO: use RedisFeatureStore
	config := ld.DefaultConfig

	ldClient, err := ld.MakeCustomClient(os.Getenv("LAUNCHDARKLY_SDK_KEY"), config, 5*time.Second)
	if err != nil {
		log.Printf("Failed to initialize LaunchDarkly client: %s", err)
		return &events.APIGatewayProxyResponse{StatusCode: http.StatusInternalServerError}, nil
	}
	defer ldClient.Close()

	log.Print("Updated flag store")

	return &events.APIGatewayProxyResponse{StatusCode: http.StatusOK}, nil
}
