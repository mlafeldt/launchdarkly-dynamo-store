package dynamodb_test

import (
	"os"
	"testing"
	"time"

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

func TestDynamoDBFeatureStore_UseLdd(t *testing.T) {
	table := os.Getenv(envTable)
	if table == "" {
		t.Skipf("%s not set in environment", envTable)
	}

	store, err := dynamodb.NewDynamoDBFeatureStore(table, nil)
	if err != nil {
		t.Fatal(err)
	}

	config := ld.DefaultConfig
	config.FeatureStore = store
	config.UseLdd = true // Enable daemon mode to only read flags from DynamoDB

	ldClient, err := ld.MakeCustomClient("some-invalid-sdk-key", config, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer ldClient.Close()

	// Since we're using an invalid SDK key, this should return the default value
	ldUser := ld.NewUser("dynamo-store-test")
	flag, err := ldClient.IntVariation("some.unknown.feature", ldUser, 2018)
	if err != nil {
		t.Log(err)
	}
	if flag != 2018 {
		t.Fatal("flag not set to default value")
	}
}
