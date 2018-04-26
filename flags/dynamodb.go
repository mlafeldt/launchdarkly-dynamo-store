package main

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	ld "gopkg.in/launchdarkly/go-client.v3"
)

type DynamoDBFeatureStore struct {
	client    *dynamodb.DynamoDB
	tableName string
	logger    ld.Logger
}

func NewDynamoDBFeatureStore(tableName string) (*DynamoDBFeatureStore, error) {
	logger := log.New(os.Stderr, "[LaunchDarkly DynamoDBFeatureStore]", log.LstdFlags)

	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	client := dynamodb.New(sess)

	info, err := client.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return nil, err
	}
	logger.Printf("DynamoDB table = %s", info.Table)

	return &DynamoDBFeatureStore{
		client:    client,
		tableName: tableName,
		logger:    logger,
	}, nil
}

func (store *DynamoDBFeatureStore) Get(kind ld.VersionedDataKind, key string) (ld.VersionedData, error) {
	// TODO
	return nil, nil
}

func (store *DynamoDBFeatureStore) All(kind ld.VersionedDataKind) (map[string]ld.VersionedData, error) {
	// TODO
	return nil, nil
}

func (store *DynamoDBFeatureStore) Init(map[ld.VersionedDataKind]map[string]ld.VersionedData) error {
	// TODO
	return nil
}

func (store *DynamoDBFeatureStore) Delete(kind ld.VersionedDataKind, key string, version int) error {
	// TODO
	return nil
}

func (store *DynamoDBFeatureStore) Upsert(kind ld.VersionedDataKind, item ld.VersionedData) error {
	// TODO
	return nil
}

func (store *DynamoDBFeatureStore) Initialized() bool {
	// TODO
	return false
}
