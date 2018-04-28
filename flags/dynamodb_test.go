package main

import (
	"os"
	"testing"

	ld "gopkg.in/launchdarkly/go-client.v3"
	ldtest "gopkg.in/launchdarkly/go-client.v3/shared_test"
)

type StoreBuilder struct {
	t           *testing.T
	tablePrefix string
}

func (builder *StoreBuilder) Build() ld.FeatureStore {
	store, err := NewDynamoDBFeatureStore(builder.tablePrefix)
	if err != nil {
		builder.t.Fatal(err)
	}
	return store
}

func TestDynamoDBFeatureStore(t *testing.T) {
	tablePrefix := os.Getenv("DYNAMODB_TABLE_PREFIX")
	if tablePrefix == "" {
		t.Skip("DYNAMODB_TABLE_PREFIX not set in environment")
	}

	builder := StoreBuilder{t, tablePrefix}
	ldtest.RunFeatureStoreTests(t, builder.Build)
}
