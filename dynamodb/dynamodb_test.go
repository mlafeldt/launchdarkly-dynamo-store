package dynamodb_test

import (
	"os"
	"testing"

	ld "gopkg.in/launchdarkly/go-client.v3"
	ldtest "gopkg.in/launchdarkly/go-client.v3/shared_test"

	"github.com/mlafeldt/serverless-ldr/dynamodb"
)

// StoreBuilder allows us to access the testing context from within the
// function passed to RunFeatureStoreTests.
type StoreBuilder struct {
	t     *testing.T
	table string
}

func (builder *StoreBuilder) Build() ld.FeatureStore {
	store, err := dynamodb.NewDynamoDBFeatureStore(builder.table, nil)
	if err != nil {
		builder.t.Fatal(err)
	}
	return store
}

func TestDynamoDBFeatureStore(t *testing.T) {
	table := os.Getenv("LAUNCHDARKLY_DYNAMODB_TABLE")
	if table == "" {
		t.Skip("LAUNCHDARKLY_DYNAMODB_TABLE not set in environment")
	}

	builder := StoreBuilder{t, table}
	ldtest.RunFeatureStoreTests(t, builder.Build)
}
