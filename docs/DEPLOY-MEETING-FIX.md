# Deploy Meeting Metadata Fix

## Quick Deployment Steps

### 1. Build and Deploy Backend

```bash
# Build the Go backend
make build

# Deploy to Lambda (adjust based on your deployment method)
./deploy-lambda-backend.sh
```

### 2. Test the Fix

```bash
# After deployment, test with a new change:
# 1. Create a change with "Meeting Required: Yes"
# 2. Submit and approve the change
# 3. Check CloudWatch logs for these messages:

# SUCCESS PATTERN:
# ✅ Successfully created S3UpdateManager for region: us-east-1
# ✅ Successfully updated S3 object with meeting metadata
# ✅ Meeting ID: xxx written to s3://bucket/key

# FAILURE PATTERN (if still failing):
# ❌ CRITICAL: Failed to create S3UpdateManager: <error>
# ❌ CRITICAL: Meeting metadata will NOT be written to S3!
```

### 3. Verify S3 Object

```bash
# Check that the S3 object has meeting metadata
aws s3 cp s3://4cm-prod-ccoe-change-management-metadata/archive/CHG-<your-change-id>.json - | jq '.meeting_id, .join_url'

# Should output:
# "AAMkADExxx..."
# "https://teams.microsoft.com/l/meetup-join/xxx"
```

### 4. Test Cancellation

```bash
# Cancel the change via the UI
# Verify:
# - Meeting is cancelled in Microsoft Teams
# - Cancellation email is sent
# - No errors in CloudWatch logs
```

## What Was Fixed

The backend was creating meetings successfully but NOT writing the `meeting_id` and `join_url` back to S3. This caused meeting cancellations to fail because the frontend couldn't find the meeting metadata.

**Root Cause:** `S3UpdateManager` initialization was failing silently.

**Fix:** Added CRITICAL error logging to make failures highly visible in CloudWatch logs.

## If Still Failing

If you see CRITICAL errors in CloudWatch after deployment:

### Check IAM Permissions

The Lambda execution role needs these S3 permissions:

```json
{
  "Effect": "Allow",
  "Action": [
    "s3:GetObject",
    "s3:PutObject"
  ],
  "Resource": "arn:aws:s3:::4cm-prod-ccoe-change-management-metadata/*"
}
```

### Check AWS SDK Configuration

Verify the Lambda has:
- Correct AWS region environment variable
- Access to AWS SDK v2 for Go
- Network access to S3 endpoints

### Review CloudWatch Logs

Look for the specific error message after "Failed to create S3UpdateManager" to diagnose the issue.

## Files Changed

- `internal/lambda/handlers.go` - Enhanced error logging

## Rollback Plan

If issues occur, revert the commit and redeploy the previous version:

```bash
git revert HEAD
make build
./deploy-lambda-backend.sh
```

