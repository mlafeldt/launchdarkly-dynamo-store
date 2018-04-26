package main

import (
	"os"
	"testing"

	ld "gopkg.in/launchdarkly/go-client.v3"
	ldtest "gopkg.in/launchdarkly/go-client.v3/shared_test"
)

type StoreBuilder struct {
	t         *testing.T
	tableName string
}

func (builder *StoreBuilder) Build() ld.FeatureStore {
	store, err := NewDynamoDBFeatureStore(builder.tableName)
	if err != nil {
		builder.t.Fatal(err)
	}
	return store
}

func TestDynamoDBFeatureStore(t *testing.T) {
	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		t.Skip("DYNAMODB_TABLE not set in environment")
	}

	builder := StoreBuilder{t, tableName}
	ldtest.RunFeatureStoreTests(t, builder.Build)
}
