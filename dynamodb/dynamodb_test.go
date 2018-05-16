package dynamodb_test

import (
	"os"
	"testing"

	ld "gopkg.in/launchdarkly/go-client.v4"
	ldtest "gopkg.in/launchdarkly/go-client.v4/shared_test"

	"github.com/mlafeldt/launchdarkly-dynamo-store/dynamodb"
)

const envTable = "LAUNCHDARKLY_DYNAMODB_TABLE"

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
	table := os.Getenv(envTable)
	if table == "" {
		t.Skipf("%s not set in environment", envTable)
	}

	builder := StoreBuilder{t, table}
	ldtest.RunFeatureStoreTests(t, builder.Build)
}
