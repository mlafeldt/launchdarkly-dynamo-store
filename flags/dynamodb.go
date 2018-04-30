package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
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

	item, err := unmarshalItem(kind, result.Item)
	if err != nil {
		store.logger.Printf("ERR: Failed to unmarshal item (key=%s table=%s): %s", key, table, err)
		return nil, err
	}

	if item.IsDeleted() {
		store.logger.Printf("WARN: Attempted to get deleted item (key=%s table=%s)", key, table)
		return nil, nil
	}

	return item, nil
}

func (store *DynamoDBFeatureStore) All(kind ld.VersionedDataKind) (map[string]ld.VersionedData, error) {
	table := store.tableName(kind.GetNamespace())
	var items []map[string]*dynamodb.AttributeValue

	err := store.client.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String(table),
	}, func(out *dynamodb.ScanOutput, lastPage bool) bool {
		items = append(items, out.Items...)
		return !lastPage
	})
	if err != nil {
		store.logger.Printf("ERR: Failed to scan pages (table=%s): %s", table, err)
		return nil, err
	}

	results := make(map[string]ld.VersionedData)

	for _, i := range items {
		item, err := unmarshalItem(kind, i)
		if err != nil {
			store.logger.Printf("ERR: Failed to unmarshal item (table=%s): %s", table, err)
			return nil, err
		}
		if !item.IsDeleted() {
			results[item.GetKey()] = item
		}
	}

	return results, nil
}

func (store *DynamoDBFeatureStore) Init(allData map[ld.VersionedDataKind]map[string]ld.VersionedData) error {
	for kind, items := range allData {
		table := store.tableName(kind.GetNamespace())

		if err := store.truncate(kind); err != nil {
			store.logger.Printf("ERR: Failed to delete all items (table=%s): %s", table, err)
			return err
		}

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
	deletedItem := kind.MakeDeletedItem(key, version)
	return store.updateWithVersioning(kind, deletedItem)
}

func (store *DynamoDBFeatureStore) Upsert(kind ld.VersionedDataKind, item ld.VersionedData) error {
	return store.updateWithVersioning(kind, item)
}

func (store *DynamoDBFeatureStore) Initialized() bool {
	return store.initialized
}

func (store *DynamoDBFeatureStore) tableName(namespace string) string {
	return store.tablePrefix + "-" + namespace
}

func (store *DynamoDBFeatureStore) updateWithVersioning(kind ld.VersionedDataKind, item ld.VersionedData) error {
	table := store.tableName(kind.GetNamespace())

	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		store.logger.Printf("ERR: Failed to marshal item (key=%s table=%s): %s", item.GetKey(), table, err)
		return err
	}
	_, err = store.client.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String(table),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(#key) or :version > #version"),
		ExpressionAttributeNames: map[string]*string{
			"#key":     aws.String("key"),
			"#version": aws.String("version"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":version": &dynamodb.AttributeValue{N: aws.String(strconv.Itoa(item.GetVersion()))},
		},
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
			return nil
		}
		store.logger.Printf("ERR: Failed to put item (key=%s table=%s): %s", item.GetKey(), table, err)
		return err
	}

	return nil
}

// FIXME: use BatchWriteItem etc. to speed this up
func (store *DynamoDBFeatureStore) truncate(kind ld.VersionedDataKind) error {
	table := store.tableName(kind.GetNamespace())

	var items []map[string]*dynamodb.AttributeValue

	err := store.client.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String(table),
	}, func(out *dynamodb.ScanOutput, lastPage bool) bool {
		items = append(items, out.Items...)
		return !lastPage
	})
	if err != nil {
		store.logger.Printf("ERR: Failed to scan pages (table=%s): %s", table, err)
		return err
	}

	for _, i := range items {
		item, err := unmarshalItem(kind, i)
		if err != nil {
			store.logger.Printf("ERR: Failed to unmarshal item (table=%s): %s", table, err)
			return err
		}

		_, err = store.client.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(table),
			Key: map[string]*dynamodb.AttributeValue{
				PrimaryPartitionKey: {S: aws.String(item.GetKey())},
			},
		})
		if err != nil {
			store.logger.Printf("ERR: Failed to delete item (key=%s table=%s): %s", item.GetKey(), table, err)
			return err
		}
	}

	return nil
}

func unmarshalItem(kind ld.VersionedDataKind, item map[string]*dynamodb.AttributeValue) (ld.VersionedData, error) {
	data := kind.GetDefaultItem()
	if err := dynamodbattribute.UnmarshalMap(item, &data); err != nil {
		return nil, err
	}
	if item, ok := data.(ld.VersionedData); ok {
		return item, nil
	}
	return nil, fmt.Errorf("Unexpected data type from unmarshal: %T", data)
}
