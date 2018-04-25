package main

import (
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handler)
}

func handler(req *events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	log.Printf("webhook payload = %s", req.Body)

	// TODO: populate Redis flag store

	return &events.APIGatewayProxyResponse{StatusCode: 200}, nil
}
