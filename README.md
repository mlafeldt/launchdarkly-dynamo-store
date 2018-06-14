# DynamoDB Store for LaunchDarkly's Go SDK

[![CircleCI](https://circleci.com/gh/mlafeldt/launchdarkly-dynamo-store.svg?style=svg&circle-token=5bd6fb4a2be7d94577cc359c6a74235aed4adc74)](https://circleci.com/gh/mlafeldt/launchdarkly-dynamo-store)
[![GoDoc](https://godoc.org/github.com/mlafeldt/launchdarkly-dynamo-store/dynamodb?status.svg)](https://godoc.org/github.com/mlafeldt/launchdarkly-dynamo-store/dynamodb)

This project provides the building blocks that, taken together, allow you to create a *serverless flag storage pipeline for LaunchDarkly* as described in [this presentation](https://speakerdeck.com/mlafeldt/implementing-feature-flags-in-serverless-environments).

By caching feature flag data in DynamoDB, LaunchDarkly clients don't need to call out to the LaunchDarkly API every time they're created. This is useful for environments like AWS Lambda where workloads can be sensitive to cold starts.

To that end, the following building blocks are provided:

- [A DynamoDB-backed feature store](https://godoc.org/github.com/mlafeldt/launchdarkly-dynamo-store/dynamodb) for the [LaunchDarkly Go SDK](https://github.com/launchdarkly/go-client).
- [A serverless service](serverless.yml) to persist feature flag data from LaunchDarkly in DynamoDB. See below for details.
- [An example Lambda function](_examples/lambda) that reads feature flags from DynamoDB without querying the LaunchDarkly API.

## Architecture

![](pipeline.png)

## The Serverless Service

The service is based on the [Serverless Framework](https://serverless.com/framework/). In addition to the `serverless` command-line tool, you can use the accompanied Makefile for convenience.

Here's how to deploy and operate the service in AWS:

```bash
# Set AWS credentials and region
$ export AWS_ACCESS_KEY_ID=...
$ export AWS_SECRET_ACCESS_KEY=...
$ export AWS_REGION=...

# Write your LaunchDarkly SDK key to the AWS Parameter Store. The service uses
# this key to talk to the LaunchDarkly API, but really any client might use it.
$ aws ssm put-parameter --name /launchdarkly/staging/sdkkey --value $SDK_KEY --type SecureString

# Deploy a service that handles feature flags for the staging environment
$ make deploy ENV=staging
$ make staging  # shortcut

# Invoke the service manually
$ serverless invoke --function store --stage staging

# Print the webhook URL (see "LaunchDarkly Webhook Configuration" below)
$ make url ENV=staging

# Show service logs
$ make logs-store ENV=staging

# Remove the service and its resources from AWS
$ make destroy ENV=staging
```

To set up a service for caching production flags, replace all occurrences of `staging` with `production`.

Also note that `staging` is the default environment, which means you may omit `ENV=staging`.

## LaunchDarkly Webhook Configuration

We want LaunchDarkly to invoke our serverless service every time a feature flag (or segment) is modified. This ensures that the data cached in DynamoDB stays up-to-date.

To achieve this, we need to set up a webhook in LaunchDarkly (listed under *Integrations*). The webhook configuration is straightforward: paste the output of `make url` into the *URL* field and use  the following JSON document as the *Policy*:

```json
[
  {
    "resources": [
      "proj/*:env/staging:flag/*"
    ],
    "actions": [
      "*"
    ],
    "effect": "allow"
  },
  {
    "resources": [
      "proj/*:env/staging:segment/*"
    ],
    "actions": [
      "*"
    ],
    "effect": "allow"
  }
]
```

(For production, replace `staging` accordingly.)

## Optional: Webhook Signature Verification

LaunchDarkly can also sign webhook payloads so you can verify that requests are generated by LaunchDarkly and not some rogue third party.

To enable webhook signature verification, configure a *Secret* in the LaunchDarkly UI. Then write that same secret to the Parameter Store and redeploy the serverless service for it to validate all future webhook requests:

```bash
$ aws ssm put-parameter --name /launchdarkly/staging/webhooksecret --value $SECRET --type SecureString
$ make staging
```

(For production, replace `staging` accordingly.)

## Author

This project is being developed by [Mathias Lafeldt](https://twitter.com/mlafeldt).
