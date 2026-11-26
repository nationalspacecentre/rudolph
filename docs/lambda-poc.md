# Single-device Santa proof of concept

## Build and verify

Use Go 1.27.4 or a later supported patch release, Bash, and Info-ZIP. ZIP verification also uses `file`.

```sh
go test ./...
go vet ./...
go install golang.org/x/vuln/cmd/govulncheck@v1.10.0
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 govulncheck -tags=lambda.norpc -test -scan=package ./...
make build
./scripts/verify-lambda-zips.sh
```

`make build` creates `build/package/api_deployment.zip` and `build/package/api_authorizer_deployment.zip`. Each contains one executable named `bootstrap` at its root. Both use static Linux ARM64 binaries and the `lambda.norpc` build tag for the Lambda Runtime API. The build also produces the universal CLI on macOS and a native CLI on Linux. GitHub Actions runs the build and checks on pushes and pull requests, then uploads the two ZIPs in the `rudolph-lambda-linux-arm64` artifact. The workflow uses `contents: read` and requires no AWS credentials. [AWS Go ZIP packaging guide](https://docs.aws.amazon.com/lambda/latest/dg/golang-package.html)

## API Gateway and Lambda

One API Lambda can serve the four Santa v1 JSON sync stages. The existing router matches the exact REST API Gateway resource template and HTTP method, and each route initializes its own dependencies when first invoked. Point these explicit REST API `AWS_PROXY` integrations at the same API Lambda. Set the Lambda runtime to `provided.al2023`, architecture to `arm64`, and handler to `bootstrap`. [REST API Lambda proxy event format](https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html) · [Santa sync protocol](https://santa.dev/development/sync-protocol.html)

| Method | Resource | Purpose |
| --- | --- | --- |
| POST | `/preflight/{machine_id}` | Report sensor state and retrieve configuration |
| POST | `/eventupload/{machine_id}` | Receive execution events |
| POST | `/ruledownload/{machine_id}` | Return rules and pagination cursor |
| POST | `/postflight/{machine_id}` | Finalize sync state |
| GET | `/health` | Check DynamoDB connectivity |
| POST | `/xsrf/{machine_id}` | Legacy stub that returns success |

The proxy event must provide `resource`, `httpMethod`, `pathParameters.machine_id` for sync routes, the JSON content type, and the body as a JSON text string. A catch-all `/{proxy+}` event or HTTP API payload format 2.0 falls outside the router's contract. The existing authorizer expects the REST API REQUEST event shape and runs as a separate Lambda if used. The legacy `logupload` endpoint has no handler. [REST API proxy event format](https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html) · [HTTP API payload formats](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html)

In the private HCP Terraform deployment for `eu-west-2`, set `REGION` and `DYNAMODB_NAME` on the API Lambda. Its role needs the DynamoDB permissions used by the combined routes and permissions for any configured event destination. Select `HANDLER=FIREHOSE` with `FIREHOSE_NAME`, `HANDLER=KINESIS` with `KINESIS_NAME`, or `HANDLER=NONE`. Setting `LAMBDA_NAME` enables additional Lambda forwarding. Without a destination, accepted uploads are discarded. The optional separate authorizer reads `REGION`, `GATEWAY_ID`, and `ACCOUNT_ID`. The legacy example Terraform in this repository still declares `provided.al2`.

## Logs and compatibility

The Lambda logs contain JSON operational messages such as matched route template, method, invocation ID, status, authorizer decision, and event forwarding failure or retry counts. AWS and JSON decoder error messages are kept out of request handling logs and internal-error response bodies. Event data still goes to the configured forwarding destination. Keep API Gateway body tracing and downstream payload logging disabled if those payloads should stay out of CloudWatch.

The upstream authorizer checks request shape and uses the machine ID in its policy response. Plan device authentication and access restrictions in the private deployment and preserve existing gateway controls. The application continues to use the Santa v1 JSON shapes and DynamoDB schema. Firehose retries failed records only and returns an internal error if they still fail. Kinesis keeps its existing single-request behavior and does not inspect per-record failures in successful API responses. CEL and FAA are outside this proof of concept.

Use the intended Santa release for an end-to-end sync through the real gateway. Include a clean sync, a paginated rule download, postflight, and event forwarding if configured. Both extracted bootstraps passed an offline smoke test in the official ARM64 Lambda AL2023 base image. The API handled a complete sync, including a pagination cursor, and the authorizer returned the expected allow and deny policies. The test used a local Runtime API and service stubs for DynamoDB, Firehose, and Kinesis.
