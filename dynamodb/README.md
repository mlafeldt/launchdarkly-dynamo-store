# A DynamoDB-backed feature store for LaunchDarkly

## Usage

```golang
import ld "gopkg.in/launchdarkly/go-client.v3"

store, err := dynamodb.NewDynamoDBFeatureStore("some-table-", nil)
if err != nil { ... }

config := ld.DefaultConfig
config.FeatureStore = store

ldClient, err := ld.MakeCustomClient("SOME_SDK_KEY", config, 5*time.Second)
if err != nil { ... }
```
