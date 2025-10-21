# Backend Role ARN as User ID

## Problem
When the backend Lambda updates S3 objects with meeting information, it was using a generic string `"backend-system"` as the `user_id` in modification entries. This made it difficult to identify which specific Lambda role performed the action.

## Solution
Updated the system to use the actual IAM Role ARN of the backend Lambda as the `user_id` instead of the generic string.

## Changes Made

### 1. Updated User ID Validation
**File:** `internal/types/types.go`

Added support for IAM Role ARN format in user ID validation:

```go
// ValidateUserIDFormat validates that user_id follows expected format patterns
func ValidateUserIDFormat(userID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user_id cannot be empty")
	}

	// Allow backend system user ID (deprecated, but still supported for backward compatibility)
	if userID == BackendUserID {
		return nil
	}

	// Allow IAM Role ARN format (preferred for backend system)
	// Format: arn:aws:iam::123456789012:role/role-name
	if strings.HasPrefix(userID, "arn:aws:iam::") && strings.Contains(userID, ":role/") {
		return nil
	}

	// Validate Identity Center user ID format (UUID-like format)
	// ... rest of validation
}
```

Deprecated the old constant:

```go
// Backend user ID for system-generated modifications
// Deprecated: Use the actual Lambda execution role ARN instead
const BackendUserID = "backend-system"
```

### 2. Updated ModificationManager
**File:** `internal/lambda/modifications.go`

Modified `NewModificationManager()` to automatically retrieve and use the backend role ARN:

```go
// NewModificationManager creates a new ModificationManager with backend role ARN from environment
func NewModificationManager() *ModificationManager {
	// Try to get the backend role ARN from environment
	backendUserID := getBackendRoleARNFromEnv()
	if backendUserID == "" {
		// Fallback to legacy constant if no role ARN is available
		log.Printf("⚠️  No backend role ARN found in environment, using legacy backend-system ID")
		backendUserID = types.BackendUserID
	} else {
		log.Printf("✅ Using backend role ARN as user ID: %s", backendUserID)
	}
	
	return &ModificationManager{
		BackendUserID: backendUserID,
	}
}
```

## Environment Variables

The system checks for the backend role ARN in the following environment variables (in order):

1. `BACKEND_ROLE_ARN` (preferred)
2. `AWS_LAMBDA_ROLE_ARN`
3. `LAMBDA_EXECUTION_ROLE_ARN`

If none are set, it falls back to the legacy `"backend-system"` string for backward compatibility.

## Example Output

### Before:
```json
{
  "modifications": [
    {
      "timestamp": "2025-01-15T15:00:00Z",
      "user_id": "backend-system",
      "modification_type": "meeting_scheduled",
      "meeting_metadata": {
        "meeting_id": "AAMkAGVm...",
        "join_url": "https://teams.microsoft.com/..."
      }
    }
  ]
}
```

### After:
```json
{
  "modifications": [
    {
      "timestamp": "2025-01-15T15:00:00Z",
      "user_id": "arn:aws:iam::123456789012:role/ccoe-customer-contact-manager-backend-lambda-role",
      "modification_type": "meeting_scheduled",
      "meeting_metadata": {
        "meeting_id": "AAMkAGVm...",
        "join_url": "https://teams.microsoft.com/..."
      }
    }
  ]
}
```

## Benefits

1. **Traceability**: Can identify exactly which Lambda role performed the action
2. **Security Auditing**: Better audit trail for compliance and security reviews
3. **Multi-Environment Support**: Different environments (dev, staging, prod) will have different role ARNs
4. **AWS Integration**: Role ARNs can be used with AWS CloudTrail and other AWS services for correlation
5. **Backward Compatibility**: Falls back to legacy string if role ARN is not available

## Deployment Requirements

To use this feature, ensure the Lambda function has one of these environment variables set:

```bash
BACKEND_ROLE_ARN=arn:aws:iam::123456789012:role/ccoe-customer-contact-manager-backend-lambda-role
```

Or in Terraform:

```hcl
resource "aws_lambda_function" "backend" {
  # ... other configuration ...
  
  environment {
    variables = {
      BACKEND_ROLE_ARN = aws_iam_role.lambda_execution_role.arn
    }
  }
}
```

## Testing

The system will log which user ID it's using:

- ✅ `Using backend role ARN as user ID: arn:aws:iam::123456789012:role/...`
- ⚠️  `No backend role ARN found in environment, using legacy backend-system ID`

Check CloudWatch logs to verify the correct behavior.

## Related Files

- `internal/types/types.go` - Type definitions and validation
- `internal/lambda/modifications.go` - Modification manager
- `internal/lambda/user_identity.go` - Role ARN retrieval functions
- `internal/lambda/handlers.go` - Lambda handler (also has role ARN functions)

## Backward Compatibility

The system maintains full backward compatibility:
- Old modification entries with `"backend-system"` remain valid
- If no role ARN is configured, falls back to `"backend-system"`
- Validation accepts both formats
- No migration of existing data is required
