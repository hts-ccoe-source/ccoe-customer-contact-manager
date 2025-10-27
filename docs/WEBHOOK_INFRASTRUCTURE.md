# Typeform Webhook Infrastructure

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Typeform Service                             │
│                    (Survey Response Submitted)                       │
└────────────────────────────┬────────────────────────────────────────┘
                             │
                             │ POST /webhooks/typeform
                             │ Headers: Typeform-Signature
                             │ Body: Survey response JSON
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      API Gateway REST API                            │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Resource: /webhooks/typeform                                 │  │
│  │  Method: POST                                                 │  │
│  │  Integration: Lambda Proxy                                    │  │
│  │  Stage: prod                                                  │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                       │
│  Features:                                                            │
│  • CloudWatch access logging                                         │
│  • X-Ray tracing                                                     │
│  • Regional endpoint                                                 │
└────────────────────────────┬─────────────────────────────────────────┘
                             │
                             │ Lambda Proxy Integration
                             │ (Full request/response control)
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Webhook Lambda Function                           │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │  Runtime: Go (provided.al2)                                   │  │
│  │  Architecture: ARM64 (Graviton)                               │  │
│  │  Timeout: 30 seconds                                          │  │
│  │  Memory: 256 MB                                               │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                       │
│  Processing Steps:                                                    │
│  1. Extract Typeform-Signature header                                │
│  2. Retrieve webhook secret from Parameter Store                     │
│  3. Validate HMAC-SHA256 signature                                   │
│  4. Parse webhook payload                                            │
│  5. Extract hidden fields (customer_code, year, quarter, etc.)       │
│  6. Store results in S3                                              │
└────────┬────────────────────┬────────────────────┬───────────────────┘
         │                    │                    │
         │                    │                    │
         ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────────┐
│ SSM Parameter   │  │   S3 Bucket     │  │   CloudWatch Logs       │
│     Store       │  │                 │  │                         │
├─────────────────┤  ├─────────────────┤  ├─────────────────────────┤
│ • Webhook       │  │ surveys/        │  │ • Lambda logs           │
│   Secret        │  │   results/      │  │ • API Gateway logs      │
│   (encrypted)   │  │   {customer}/   │  │ • Error tracking        │
│                 │  │   {year}/       │  │ • Performance metrics   │
│                 │  │   {quarter}/    │  │                         │
│                 │  │   {ts}-{id}.json│  │                         │
└─────────────────┘  └─────────────────┘  └─────────────────────────┘
```

## Request Flow

### 1. Typeform Sends Webhook

```http
POST https://abc123.execute-api.us-east-1.amazonaws.com/prod/webhooks/typeform
Content-Type: application/json
Typeform-Signature: sha256=abc123def456...

{
  "event_id": "01HGWQR...",
  "event_type": "form_response",
  "form_response": {
    "form_id": "HLjqXS5W",
    "token": "abc123",
    "submitted_at": "2025-10-25T14:30:00Z",
    "hidden": {
      "user_login": "john.doe@hearst.com",
      "customer_code": "ACME",
      "year": "2025",
      "quarter": "Q4",
      "event_type": "change",
      "event_subtype": "general"
    },
    "answers": [...]
  }
}
```

### 2. API Gateway Routes to Lambda

API Gateway performs:
- Request validation
- CloudWatch logging
- X-Ray tracing
- Lambda proxy integration

### 3. Lambda Validates and Processes

```go
// 1. Extract signature
signature := request.Headers["Typeform-Signature"]

// 2. Retrieve secret from Parameter Store
secret := getParameter("/hts/.../TYPEFORM_WEBHOOK_SECRET")

// 3. Validate HMAC
if !validateSignature(payload, signature, secret) {
    return 401 Unauthorized
}

// 4. Parse payload
webhook := parseWebhook(payload)

// 5. Extract metadata
customerCode := webhook.FormResponse.Hidden["customer_code"]
year := webhook.FormResponse.Hidden["year"]
quarter := webhook.FormResponse.Hidden["quarter"]

// 6. Store in S3
key := fmt.Sprintf("surveys/results/%s/%s/%s/%d-%s.json",
    customerCode, year, quarter, timestamp, formID)
s3.PutObject(bucket, key, payload)

// 7. Return success
return 200 OK
```

### 4. Response to Typeform

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "success"
}
```

## Error Handling

### Invalid Signature (401)

```
Typeform → API Gateway → Lambda
                           ↓
                    Validate Signature
                           ↓
                    ❌ Invalid
                           ↓
                    Return 401
                           ↓
                    CloudWatch Log
```

### S3 Storage Failure (500)

```
Lambda → S3 PutObject
           ↓
      ❌ Error
           ↓
    Retry (3x with backoff)
           ↓
      Still fails
           ↓
    Return 500
           ↓
    CloudWatch Log + Alarm
```

## IAM Permissions

### Webhook Lambda Role

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ],
      "Resource": "arn:aws:logs:*:*:log-group:/aws/lambda/*-webhook*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject"
      ],
      "Resource": "arn:aws:s3:::bucket/surveys/results/*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ssm:GetParameter",
        "ssm:GetParameters"
      ],
      "Resource": "arn:aws:ssm:*:*:parameter/hts/.../TYPEFORM_WEBHOOK_SECRET"
    }
  ]
}
```

### API Gateway Invoke Permission

```json
{
  "Effect": "Allow",
  "Principal": {
    "Service": "apigateway.amazonaws.com"
  },
  "Action": "lambda:InvokeFunction",
  "Resource": "arn:aws:lambda:*:*:function:*-webhook",
  "Condition": {
    "ArnLike": {
      "AWS:SourceArn": "arn:aws:execute-api:*:*:*/*/POST/webhooks/typeform"
    }
  }
}
```

## Monitoring

### CloudWatch Metrics

#### Lambda Metrics
- **Invocations** - Total webhook requests processed
- **Errors** - Failed webhook processing
- **Duration** - Processing time per request
- **Throttles** - Rate limiting events

#### API Gateway Metrics
- **Count** - Total API requests
- **4XXError** - Client errors (invalid signatures)
- **5XXError** - Server errors (Lambda failures)
- **Latency** - End-to-end request time

### CloudWatch Alarms

```
┌─────────────────────────────────────────────────────────────┐
│                    CloudWatch Alarms                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Webhook Lambda Errors                                       │
│  ├─ Threshold: 5 errors in 5 minutes                        │
│  ├─ Action: SNS notification                                │
│  └─ Severity: Critical                                      │
│                                                              │
│  API Gateway 5xx Errors                                      │
│  ├─ Threshold: 5 errors in 5 minutes                        │
│  ├─ Action: SNS notification                                │
│  └─ Severity: Critical                                      │
│                                                              │
│  API Gateway 4xx Errors                                      │
│  ├─ Threshold: 10 errors in 5 minutes                       │
│  ├─ Action: SNS notification                                │
│  └─ Severity: Warning (includes invalid signatures)         │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Log Insights Queries

#### Find Invalid Signatures
```
fields @timestamp, @message
| filter @message like /invalid webhook signature/
| sort @timestamp desc
| limit 100
```

#### Find S3 Storage Failures
```
fields @timestamp, @message
| filter @message like /failed to store/
| sort @timestamp desc
| limit 100
```

#### Calculate Success Rate
```
fields @timestamp
| stats count() as total,
        sum(statusCode = 200) as success,
        sum(statusCode = 401) as unauthorized,
        sum(statusCode = 500) as errors
by bin(5m)
```

## Security

### HMAC Signature Validation

```
┌─────────────────────────────────────────────────────────────┐
│                  HMAC Validation Process                     │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. Typeform computes signature:                            │
│     signature = HMAC-SHA256(payload, secret)                │
│                                                              │
│  2. Typeform sends request:                                 │
│     Header: Typeform-Signature: sha256={signature}          │
│                                                              │
│  3. Lambda retrieves secret from Parameter Store            │
│                                                              │
│  4. Lambda computes expected signature:                     │
│     expected = HMAC-SHA256(payload, secret)                 │
│                                                              │
│  5. Lambda compares signatures:                             │
│     if signature == expected:                               │
│         process webhook                                     │
│     else:                                                   │
│         return 401 Unauthorized                             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Secret Management

```
┌─────────────────────────────────────────────────────────────┐
│              SSM Parameter Store (Encrypted)                 │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Parameter: /hts/.../TYPEFORM_WEBHOOK_SECRET                │
│  Type: SecureString                                          │
│  KMS Key: AWS managed key                                   │
│  Access: Lambda execution role only                         │
│                                                              │
│  Rotation:                                                   │
│  1. Update parameter value                                  │
│  2. Update Typeform webhook configuration                   │
│  3. Lambda automatically uses new value                     │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Deployment

### Terraform Resources Created

```
terraform apply
  ↓
Creates:
  • aws_api_gateway_rest_api.typeform_webhook
  • aws_api_gateway_resource.webhooks
  • aws_api_gateway_resource.typeform
  • aws_api_gateway_method.typeform_post
  • aws_api_gateway_integration.typeform_lambda
  • aws_api_gateway_deployment.typeform_webhook
  • aws_api_gateway_stage.typeform_webhook
  • aws_lambda_function.webhook_handler
  • aws_iam_role.webhook_lambda_role
  • aws_iam_role_policy.webhook_lambda_policy
  • aws_lambda_permission.api_gateway_invoke_webhook
  • aws_cloudwatch_log_group.webhook_lambda_logs
  • aws_cloudwatch_log_group.api_gateway_logs
  • aws_cloudwatch_metric_alarm.webhook_lambda_errors
  • aws_cloudwatch_metric_alarm.api_gateway_5xx_errors
  • aws_cloudwatch_metric_alarm.api_gateway_4xx_errors
```

### Build Process

```
make build-webhook-lambda
  ↓
GOOS=linux GOARCH=arm64 go build -o bin/webhook-lambda ./cmd/webhook
  ↓
Terraform packages binary
  ↓
Deploys to Lambda
```

## Testing

### Local Testing (Signature Validation)

```bash
# Generate test signature
SECRET="your-webhook-secret"
PAYLOAD='{"test":"data"}'
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$SECRET" | cut -d' ' -f2)

# Test webhook
curl -X POST $WEBHOOK_URL \
  -H "Content-Type: application/json" \
  -H "Typeform-Signature: sha256=$SIGNATURE" \
  -d "$PAYLOAD"
```

### Integration Testing

```bash
# 1. Create test form in Typeform
# 2. Configure webhook URL
# 3. Submit test response
# 4. Verify in CloudWatch logs
aws logs tail /aws/lambda/ccoe-customer-contact-manager-webhook --follow

# 5. Verify in S3
aws s3 ls s3://bucket/surveys/results/ --recursive
```

## Performance

### Latency Breakdown

```
Total: ~200-500ms
├─ API Gateway: 10-20ms
├─ Lambda Cold Start: 100-200ms (first request)
├─ Lambda Warm: 10-20ms (subsequent)
├─ Parameter Store: 20-50ms
├─ HMAC Validation: 1-5ms
├─ JSON Parsing: 1-5ms
└─ S3 PutObject: 50-100ms
```

### Optimization

- **ARM64 Architecture**: 20% faster than x86_64
- **Minimal Memory**: 256 MB sufficient for webhook processing
- **Parameter Store Caching**: Cache secret for 5 minutes
- **S3 Retry Logic**: Exponential backoff for reliability

## Cost Analysis

### Monthly Cost (10,000 submissions)

```
API Gateway:
  10,000 requests × $3.50/1M = $0.035

Lambda:
  10,000 invocations × 0.5s × 256MB = 1,280 GB-seconds
  Free tier: 400,000 GB-seconds/month
  Cost: $0.00 (within free tier)

CloudWatch Logs:
  10,000 × 2KB = 20 MB ingested
  20 MB × $0.50/GB = $0.01

Total: ~$0.05/month
```

## References

- [Typeform Webhooks](https://developer.typeform.com/webhooks/)
- [API Gateway Lambda Proxy](https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html)
- [Lambda with Go](https://docs.aws.amazon.com/lambda/latest/dg/lambda-golang.html)
- [SSM Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html)
