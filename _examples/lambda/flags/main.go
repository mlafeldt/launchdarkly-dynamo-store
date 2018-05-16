package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	ld "gopkg.in/launchdarkly/go-client.v4"

	"github.com/mlafeldt/launchdarkly-dynamo-store/dynamodb"
)

func main() {
	lambda.Start(handler)
}

func handler(req *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	store, err := dynamodb.NewDynamoDBFeatureStore(os.Getenv("LAUNCHDARKLY_DYNAMODB_TABLE"), nil)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to initialize DynamoDBFeatureStore: %s\n", err),
		}, nil
	}

	config := ld.DefaultConfig
	config.FeatureStore = store
	config.UseLdd = true

	ldClient, err := ld.MakeCustomClient(os.Getenv("LAUNCHDARKLY_SDK_KEY"), config, 5*time.Second)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       fmt.Sprintf("Failed to initialize LaunchDarkly client: %s\n", err),
		}, nil
	}
	defer ldClient.Close()

	// Get and return all flags for the Lambda function
	ldUser := ld.NewUser(os.Getenv("AWS_LAMBDA_FUNCTION_NAME"))
	flags := ldClient.AllFlags(ldUser)
	jsonFlags, _ := json.Marshal(flags)

	return &events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(jsonFlags),
	}, nil
}
