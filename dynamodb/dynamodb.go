/*
Package dynamodb provides a DynamoDB-backed feature store for the LaunchDarkly
Go SDK.

By caching feature flag data in DynamoDB, LaunchDarkly clients don't need to
call out to the LaunchDarkly API every time they're created. This is useful for
environments like AWS Lambda where workloads can be sensitive to cold starts.

In contrast to the Redis-backed feature store, the DynamoDB store can be used
without requiring access to any VPC resources, i.e. ElastiCache Redis. See
https://blog.launchdarkly.com/go-serveless-not-flagless-implementing-feature-flags-in-serverless-environments/
for more background information.

Here's how to use the feature store with the LaunchDarkly client:

	store, err := dynamodb.NewDynamoDBFeatureStore("some-table-", nil)
	if err != nil { ... }

	config := ld.DefaultConfig
	config.FeatureStore = store

	ldClient, err := ld.MakeCustomClient("SOME_SDK_KEY", config, 5*time.Second)
	if err != nil { ... }

The DynamoDB tables used by the store must adhere to this simple schema:

        AttributeDefinitions:
          - AttributeName: key
            AttributeType: S
        KeySchema:
          - AttributeName: key
            KeyType: HASH
*/
package dynamodb

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

const primaryPartitionKey = "key"

// Verify that the store satisfies the FeatureStore interface
var _ ld.FeatureStore = (*DynamoDBFeatureStore)(nil)

// DynamoDBFeatureStore provides a DynamoDB-backed feature store for LaunchDarkly.
type DynamoDBFeatureStore struct {
	// Client used to access DynamoDB
	Client *dynamodb.DynamoDB

	// Prefix added to the beginning of the name of each DynamoDB table
	// used by the store
	TablePrefix string

	// All log messages will be written to this Logger
	Logger ld.Logger

	initialized bool
}

// NewDynamoDBFeatureStore creates a new DynamoDB feature store ready to be used
// by the LaunchDarkly client.
//
// This function uses https://docs.aws.amazon.com/sdk-for-go/api/aws/session/#NewSession
// to configure access to DynamoDB, which means that environment variables like
// AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, and AWS_REGION work as expected.
//
// For more control, compose your own DynamoDBFeatureStore with a custom DynamoDB client.
func NewDynamoDBFeatureStore(tablePrefix string, logger ld.Logger) (*DynamoDBFeatureStore, error) {
	if logger == nil {
		logger = log.New(os.Stderr, "[LaunchDarkly DynamoDBFeatureStore]", log.LstdFlags)
	}

	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	client := dynamodb.New(sess)

	return &DynamoDBFeatureStore{
		Client:      client,
		TablePrefix: tablePrefix,
		Logger:      logger,
		initialized: false,
	}, nil
}

// Init initializes the store by writing the given data to DynamoDB, using a
// separate table for each data kind (e.g. one table for flags and another one
// for segments). It will delete all existing data from the tables.
func (store *DynamoDBFeatureStore) Init(allData map[ld.VersionedDataKind]map[string]ld.VersionedData) error {
	for kind, items := range allData {
		table := store.tableName(kind)

		if err := store.truncateTable(kind); err != nil {
			store.Logger.Printf("ERROR: Failed to delete all items (table=%s): %s", table, err)
			return err
		}

		for k, v := range items {
			av, err := dynamodbattribute.MarshalMap(v)
			if err != nil {
				store.Logger.Printf("ERROR: Failed to marshal item (key=%s table=%s): %s", k, table, err)
				return err
			}
			_, err = store.Client.PutItem(&dynamodb.PutItemInput{
				TableName: aws.String(table),
				Item:      av,
			})
			if err != nil {
				store.Logger.Printf("ERROR: Failed to put item (key=%s table=%s): %s", k, table, err)
				return err
			}
		}
	}

	store.initialized = true

	return nil
}

// Initialized returns true if the store has been initialized.
func (store *DynamoDBFeatureStore) Initialized() bool {
	return store.initialized
}

// All returns all items currently stored in DynamoDB that are of the given
// data kind. It won't return items marked as deleted.
func (store *DynamoDBFeatureStore) All(kind ld.VersionedDataKind) (map[string]ld.VersionedData, error) {
	table := store.tableName(kind)
	var items []map[string]*dynamodb.AttributeValue

	err := store.Client.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String(table),
	}, func(out *dynamodb.ScanOutput, lastPage bool) bool {
		items = append(items, out.Items...)
		return !lastPage
	})
	if err != nil {
		store.Logger.Printf("ERROR: Failed to scan pages (table=%s): %s", table, err)
		return nil, err
	}

	results := make(map[string]ld.VersionedData)

	for _, i := range items {
		item, err := unmarshalItem(kind, i)
		if err != nil {
			store.Logger.Printf("ERROR: Failed to unmarshal item (table=%s): %s", table, err)
			return nil, err
		}
		if !item.IsDeleted() {
			results[item.GetKey()] = item
		}
	}

	return results, nil
}

// Get returns a specific item with the given key. It returns nil if the item
// does not exist or if it's marked as deleted.
func (store *DynamoDBFeatureStore) Get(kind ld.VersionedDataKind, key string) (ld.VersionedData, error) {
	table := store.tableName(kind)
	input := &dynamodb.GetItemInput{
		TableName: aws.String(table),
		Key: map[string]*dynamodb.AttributeValue{
			primaryPartitionKey: {S: aws.String(key)},
		},
	}

	result, err := store.Client.GetItem(input)
	if err != nil {
		store.Logger.Printf("ERROR: Failed to get item (key=%s table=%s): %s", key, table, err)
		return nil, err
	}

	if len(result.Item) == 0 {
		store.Logger.Printf("WARN: Item not found (key=%s table=%s)", key, table)
		return nil, nil
	}

	item, err := unmarshalItem(kind, result.Item)
	if err != nil {
		store.Logger.Printf("ERROR: Failed to unmarshal item (key=%s table=%s): %s", key, table, err)
		return nil, err
	}

	if item.IsDeleted() {
		store.Logger.Printf("WARN: Attempted to get deleted item (key=%s table=%s)", key, table)
		return nil, nil
	}

	return item, nil
}

// Upsert either creates a new item of the given data kind if it doesn't
// already exist, or updates an existing item if the given item has a higher
// version.
func (store *DynamoDBFeatureStore) Upsert(kind ld.VersionedDataKind, item ld.VersionedData) error {
	return store.updateWithVersioning(kind, item)
}

// Delete marks an item as deleted. (It won't actually remove the item from
// DynamoDB.)
func (store *DynamoDBFeatureStore) Delete(kind ld.VersionedDataKind, key string, version int) error {
	deletedItem := kind.MakeDeletedItem(key, version)
	return store.updateWithVersioning(kind, deletedItem)
}

func (store *DynamoDBFeatureStore) updateWithVersioning(kind ld.VersionedDataKind, item ld.VersionedData) error {
	table := store.tableName(kind)

	av, err := dynamodbattribute.MarshalMap(item)
	if err != nil {
		store.Logger.Printf("ERROR: Failed to marshal item (key=%s table=%s): %s", item.GetKey(), table, err)
		return err
	}
	_, err = store.Client.PutItem(&dynamodb.PutItemInput{
		TableName:           aws.String(table),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(#key) or :version > #version"),
		ExpressionAttributeNames: map[string]*string{
			"#key":     aws.String(primaryPartitionKey),
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
		store.Logger.Printf("ERROR: Failed to put item (key=%s table=%s): %s", item.GetKey(), table, err)
		return err
	}

	return nil
}

// FIXME: use BatchWriteItem etc. to speed this up
func (store *DynamoDBFeatureStore) truncateTable(kind ld.VersionedDataKind) error {
	table := store.tableName(kind)

	var items []map[string]*dynamodb.AttributeValue

	err := store.Client.ScanPages(&dynamodb.ScanInput{
		TableName: aws.String(table),
	}, func(out *dynamodb.ScanOutput, lastPage bool) bool {
		items = append(items, out.Items...)
		return !lastPage
	})
	if err != nil {
		store.Logger.Printf("ERROR: Failed to scan pages (table=%s): %s", table, err)
		return err
	}

	for _, i := range items {
		item, err := unmarshalItem(kind, i)
		if err != nil {
			store.Logger.Printf("ERROR: Failed to unmarshal item (table=%s): %s", table, err)
			return err
		}

		_, err = store.Client.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(table),
			Key: map[string]*dynamodb.AttributeValue{
				primaryPartitionKey: {S: aws.String(item.GetKey())},
			},
		})
		if err != nil {
			store.Logger.Printf("ERROR: Failed to delete item (key=%s table=%s): %s", item.GetKey(), table, err)
			return err
		}
	}

	return nil
}

func (store *DynamoDBFeatureStore) tableName(kind ld.VersionedDataKind) string {
	return store.TablePrefix + kind.GetNamespace()
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
