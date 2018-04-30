package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	ld "gopkg.in/launchdarkly/go-client.v3"
)

const PrimaryPartitionKey = "key"

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
	table := store.tableName(kind.GetNamespace())
	input := &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]*dynamodb.AttributeValue{
			PrimaryPartitionKey: {S: aws.String(key)},
		},
	}

	result, err := store.client.GetItem(input)
	if err != nil {
		store.logger.Printf("ERR: Failed to get item (key=%s table=%s): %s", key, table, err)
		return nil, err
	}

	if len(result.Item) == 0 {
		store.logger.Printf("WARN: Item not found (key=%s table=%s)", key, table)
		return nil, nil
	}

	data := kind.GetDefaultItem()
	if err := dynamodbattribute.UnmarshalMap(result.Item, &data); err != nil {
		store.logger.Printf("ERR: Failed to unmarshal item (key=%s table=%s): %s", key, table, err)
		return nil, err
	}
	item, ok := data.(ld.VersionedData)
	if !ok {
		return nil, fmt.Errorf("Unexpected data type from unmarshal: %T", data)
	}

	if item.IsDeleted() {
		store.logger.Printf("WARN: Attempted to get deleted item (key=%s table=%s)", key, table)
		return nil, nil
	}

	return item, nil
}

func (store *DynamoDBFeatureStore) All(kind ld.VersionedDataKind) (map[string]ld.VersionedData, error) {
	// TODO
	results := make(map[string]ld.VersionedData)

	return results, nil
}

func (store *DynamoDBFeatureStore) Init(allData map[ld.VersionedDataKind]map[string]ld.VersionedData) error {
	for kind, items := range allData {
		table := store.tableName(kind.GetNamespace())
		for k, v := range items {
			av, err := dynamodbattribute.MarshalMap(v)
			if err != nil {
				store.logger.Printf("ERR: Failed to marshal item (key=%s table=%s): %s", k, table, err)
				return err
			}
			_, err = store.client.PutItem(&dynamodb.PutItemInput{
				TableName: aws.String(table),
				Item:      av,
			})
			if err != nil {
				store.logger.Printf("ERR: Failed to put item (key=%s table=%s): %s", k, table, err)
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
