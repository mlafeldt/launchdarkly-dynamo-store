package dynamodb_test

import (
	"os"
	"testing"

	ld "gopkg.in/launchdarkly/go-client.v4"
	ldtest "gopkg.in/launchdarkly/go-client.v4/shared_test"

	"github.com/mlafeldt/launchdarkly-dynamo-store/dynamodb"
)

const envTable = "LAUNCHDARKLY_DYNAMODB_TABLE"

func TestDynamoDBFeatureStore(t *testing.T) {
	table := os.Getenv(envTable)
	if table == "" {
		t.Skipf("%s not set in environment", envTable)
	}

	ldtest.RunFeatureStoreTests(t, func() ld.FeatureStore {
		store, err := dynamodb.NewDynamoDBFeatureStore(table, nil)
		if err != nil {
			t.Fatal(err)
		}
		return store
	})
}
