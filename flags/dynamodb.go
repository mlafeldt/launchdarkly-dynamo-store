package main

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	ld "gopkg.in/launchdarkly/go-client.v3"
)

type DynamoDBFeatureStore struct {
	client      *dynamodb.DynamoDB
	tablePrefix string
	logger      ld.Logger
	initialized bool
}

func NewDynamoDBFeatureStore(tablePrefix string) (*DynamoDBFeatureStore, error) {
	logger := log.New(os.Stderr, "[LaunchDarkly DynamoDBFeatureStore]", log.LstdFlags)

	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	client := dynamodb.New(sess)

	return &DynamoDBFeatureStore{
		client:      client,
		tablePrefix: tablePrefix,
		logger:      logger,
		initialized: false,
	}, nil
}

func (store *DynamoDBFeatureStore) Get(kind ld.VersionedDataKind, key string) (ld.VersionedData, error) {
	// TODO
	return nil, nil
}

func (store *DynamoDBFeatureStore) All(kind ld.VersionedDataKind) (map[string]ld.VersionedData, error) {
	// TODO
	results := make(map[string]ld.VersionedData)

	return results, nil
}

func (store *DynamoDBFeatureStore) Init(allData map[ld.VersionedDataKind]map[string]ld.VersionedData) error {
	for kind, items := range allData {
		table := store.tableName(kind.GetNamespace())
		for _, v := range items {
			if err := store.putItem(table, v); err != nil {
				return err
			}
		}
	}
	store.initialized = true
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
	return store.initialized
}

func (store *DynamoDBFeatureStore) tableName(namespace string) string {
	return store.tablePrefix + "-" + namespace
}

func (store *DynamoDBFeatureStore) putItem(table string, v interface{}) error {
	av, err := dynamodbattribute.MarshalMap(v)
	if err != nil {
		return err
	}
	_, err1 := store.client.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(table),
		Item:      av,
	})
	return err1
}
