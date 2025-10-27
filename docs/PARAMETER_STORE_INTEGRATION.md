# Parameter Store Integration for Typeform

This document describes how the backend Lambda and webhook Lambda load encrypted credentials from AWS Systems Manager Parameter Store.

## Overview

Both Lambda functions now load sensitive credentials from Parameter Store instead of environment variables for better security:

1. **Backend Lambda** - Loads Typeform API token for creating surveys
2. **Webhook Lambda** - Loads Typeform webhook secret for validating webhook signatures

## Parameter Store Paths

### Backend Lambda Parameters

| Parameter | Path | Purpose |
|-----------|------|---------|
| Azure Client ID | `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_ID` | Microsoft Graph API authentication |
| Azure Client Secret | `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_CLIENT_SECRET` | Microsoft Graph API authentication |
| Azure Tenant ID | `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/AZURE_TENANT_ID` | Microsoft Graph API authentication |
| **Typeform API Token** | `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_API_TOKEN` | Typeform API authentication for creating surveys |

### Webhook Lambda Parameters

| Parameter | Path | Purpose |
|-----------|------|---------|
| **Typeform Webhook Secret** | `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_WEBHOOK_SECRET` | HMAC signature validation for webhook requests |

## Implementation Details

### Backend Lambda (`internal/lambda/handlers.go`)

The backend Lambda loads all credentials at the start of the `Handler` function:

```go
func Handler(ctx context.Context, sqsEvent events.SQSEvent) error {
    // Load all credentials from Parameter Store (Azure + Typeform)
    if err := ses.LoadAllCredentialsFromSSM(ctx); err != nil {
        log.Printf("⚠️  Warning: Failed to load credentials from Parameter Store: %v", err)
        // Don't fail the entire handler - some operations may not need these credentials
    }
    // ... rest of handler
}
```

The credentials are loaded using the `LoadAllCredentialsFromSSM` function in `internal/ses/meetings.go`:

```go
func LoadAllCredentialsFromSSM(ctx context.Context) error {
    // Load Azure credentials
    if err := loadAzureCredentialsFromSSM(ctx); err != nil {
        return fmt.Errorf("failed to load Azure credentials: %w", err)
    }

    // Load Typeform API token
    if err := loadTypeformAPITokenFromSSM(ctx); err != nil {
        return fmt.Errorf("failed to load Typeform API token: %w", err)
    }

    return nil
}
```

Once loaded, the credentials are set as environment variables and cached for the lifetime of the Lambda execution context.

### Webhook Lambda (`cmd/webhook/main.go`)

The webhook Lambda loads the secret on-demand with caching:

```go
var webhookSecretCache string

func loadWebhookSecretFromSSM(ctx context.Context) (string, error) {
    // Return cached value if already loaded
    if webhookSecretCache != "" {
        return webhookSecretCache, nil
    }

    cfg, err := awsconfig.LoadDefaultConfig(ctx)
    if err != nil {
        return "", err
    }

    client := ssm.NewFromConfig(cfg)

    // Get the parameter path from environment variable
    parameterPath := os.Getenv("TYPEFORM_WEBHOOK_SECRET_PARAMETER")
    if parameterPath == "" {
        parameterPath = "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_WEBHOOK_SECRET"
    }

    result, err := client.GetParameter(ctx, &ssm.GetParameterInput{
        Name:           aws.String(parameterPath),
        WithDecryption: aws.Bool(true),
    })
    if err != nil {
        return "", err
    }

    // Cache the secret
    webhookSecretCache = *result.Parameter.Value
    return webhookSecretCache, nil
}
```

The secret is loaded during webhook signature validation:

```go
// Load webhook secret from Parameter Store
secret, err := loadWebhookSecretFromSSM(ctx)
if err != nil {
    logger.Error("failed to load webhook secret from parameter store", "error", err)
    return createErrorResponse(500, "Internal server error", "Failed to load webhook secret"), nil
}

// Validate signature
if !typeform.ValidateWebhookSignature(payload, signature, secret) {
    return createErrorResponse(401, "Unauthorized", "Invalid webhook signature"), nil
}
```

## Terraform Configuration

The Terraform configuration already includes the necessary IAM permissions:

### Backend Lambda Permissions (`terraform/main.tf`)

```hcl
{
  Effect = "Allow"
  Action = [
    "ssm:GetParameter",
    "ssm:GetParameters"
  ]
  Resource = "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter${var.typeform_api_token_parameter}"
}
```

### Webhook Lambda Permissions (`terraform/api-gateway-webhook.tf`)

```hcl
{
  Effect = "Allow"
  Action = [
    "ssm:GetParameter",
    "ssm:GetParameters"
  ]
  Resource = "arn:aws:ssm:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:parameter${var.typeform_webhook_secret_parameter}"
}
```

## Creating the Parameters

Before deploying, you must create the encrypted parameters in Parameter Store:

```bash
# Create Typeform API token (for backend Lambda)
aws ssm put-parameter \
  --name "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_API_TOKEN" \
  --value "YOUR_TYPEFORM_API_TOKEN" \
  --type "SecureString" \
  --description "Typeform API token for creating surveys" \
  --region us-east-1

# Create Typeform webhook secret (for webhook Lambda)
aws ssm put-parameter \
  --name "/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_WEBHOOK_SECRET" \
  --value "YOUR_TYPEFORM_WEBHOOK_SECRET" \
  --type "SecureString" \
  --description "Typeform webhook secret for signature validation" \
  --region us-east-1
```

## Security Benefits

1. **Encryption at Rest**: All parameters are stored as `SecureString` type, encrypted with AWS KMS
2. **Encryption in Transit**: Parameters are decrypted by AWS and transmitted over TLS
3. **No Hardcoded Secrets**: Secrets are never committed to source control
4. **Centralized Management**: All secrets managed in one place (Parameter Store)
5. **Audit Trail**: CloudTrail logs all parameter access
6. **Rotation Support**: Parameters can be rotated without code changes

## Caching Strategy

Both Lambda functions implement caching to minimize Parameter Store API calls:

- **Backend Lambda**: Loads all credentials once per Lambda execution context (warm start reuses cached values)
- **Webhook Lambda**: Caches the webhook secret in a module-level variable (warm start reuses cached value)

This reduces latency and Parameter Store API costs while maintaining security.

## Troubleshooting

### Backend Lambda

If survey creation fails with "TYPEFORM_API_TOKEN environment variable not set":

1. Check that the parameter exists in Parameter Store
2. Verify the Lambda has `ssm:GetParameter` permission
3. Check CloudWatch logs for credential loading errors
4. Ensure the parameter path matches exactly

### Webhook Lambda

If webhook validation fails with "Failed to load webhook secret":

1. Check that the parameter exists in Parameter Store
2. Verify the Lambda has `ssm:GetParameter` permission
3. Check the `TYPEFORM_WEBHOOK_SECRET_PARAMETER` environment variable is set correctly
4. Review CloudWatch logs for detailed error messages

## Environment Variables

### Backend Lambda

No environment variables needed - parameters are loaded automatically from standard paths.

### Webhook Lambda

| Variable | Default | Purpose |
|----------|---------|---------|
| `TYPEFORM_WEBHOOK_SECRET_PARAMETER` | `/hts/std-app-prod/ccoe-customer-contact-manager/us-east-1/TYPEFORM_WEBHOOK_SECRET` | Parameter Store path for webhook secret |

The Terraform configuration sets this automatically via the `typeform_webhook_secret_parameter` variable.
