/*
Package dynamodb provides a DynamoDB-backed feature store for the LaunchDarkly
Go SDK.

By caching feature flag data in DynamoDB, it's possible to avoid calling out to
LaunchDarkly every time a client is created. This is useful for environments
like AWS Lambda that are highly concurrent and sensitive to cold starts.

See https://blog.launchdarkly.com/go-serveless-not-flagless-implementing-feature-flags-in-serverless-environments/
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
	Client      *dynamodb.DynamoDB
	TablePrefix string
	Logger      ld.Logger

	initialized bool
}

// NewDynamoDBFeatureStore creates a new DynamoDB feature store ready to be
// used by the LaunchDarkly client.
//
// Access to DynamoDB is configured via the environment variables
// AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, and AWS_REGION. For more control,
// compose your own DynamoDBFeatureStore with a custom Client.
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

// Init initializes the store by fetching feature flags and other items from
// LaunchDarkly and writing them to the corresponding table in DynamoDB. All
// existing items will be deleted prior to storing new data.
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

// All returns all items currently stored in DynamoDB.
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

// Get returns a specific item matching the given key.
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

// Upsert either creates a new item if it doesn't already exist, or updates an
// existing item if the passed item has a higher version.
func (store *DynamoDBFeatureStore) Upsert(kind ld.VersionedDataKind, item ld.VersionedData) error {
	return store.updateWithVersioning(kind, item)
}

// Delete marks an item as deleted. (It won't actually remove the item from the
// store.)
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
