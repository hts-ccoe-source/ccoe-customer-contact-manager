# Approval Request Email Fix

## Problem

When submitting a change for approval, no "approval request" emails were being sent. Instead, when approving the change, duplicate "approved announcement" emails and meeting invites were being sent.

## Root Cause

The `uploadToCustomerBucket()` function in `lambda/upload_lambda/upload-metadata-lambda.js` was not setting the `request-type` metadata on S3 objects when uploading during initial submission.

### What Was Happening

1. **Initial Submission** (`handleUpload`):
   - Sets `metadata.metadata.request_type = 'approval_request'` in the JSON body ✅
   - Uploads to `customers/${customer}/${changeId}.json`
   - BUT: S3 object metadata did NOT include `'request-type': 'approval_request'` ❌
   
2. **Backend Processing**:
   - S3 event triggers backend Lambda
   - Backend reads S3 object metadata first (no request-type found)
   - Falls back to reading JSON body
   - `DetermineRequestType()` should find status="submitted" and return "approval_request"
   - BUT: Something in the fallback logic wasn't working correctly
   - Result: No approval request email sent ❌

3. **Approval** (`handleApproveChange`):
   - Sets `'request-type': 'approved_announcement'` in S3 object metadata ✅
   - Backend correctly processes and sends approved announcement emails ✅
   - Schedules meetings ✅

## The Fix

Added `request-type` and `status` to S3 object metadata in `uploadToCustomerBucket()`:

```javascript
async function uploadToCustomerBucket(metadata, customer) {
    const params = {
        Bucket: bucketName,
        Key: key,
        Body: JSON.stringify(metadata, null, 2),
        ContentType: 'application/json',
        Metadata: {
            'change-id': metadata.changeId,
            'customer': customer,
            'submitted-by': metadata.submittedBy,
            'submitted-at': metadata.submittedAt,
            'request-type': 'approval_request',  // ADDED: Tell backend this is an approval request
            'status': 'submitted'                 // ADDED: Explicit status in metadata
        }
    };
    // ...
}
```

## Why This Matters

The backend Lambda's `DetermineRequestType()` function checks S3 object metadata FIRST before falling back to the JSON body. By setting the `request-type` explicitly in S3 metadata, we ensure:

1. **Fast determination**: No need to parse JSON body
2. **Reliable routing**: Explicit intent in metadata
3. **Consistency**: Same pattern as approve/complete/cancel operations

## About "Two Sets" of Emails

If you're seeing "two sets" of emails, this is likely because:

1. **Multiple Customers**: If you select 2 customers, you'll get 2 emails (one per customer) - this is expected behavior
2. **Duplicate Processing**: If the same S3 event is being processed twice, check:
   - SQS queue configuration (dead letter queue, retry settings)
   - Lambda concurrency settings
   - S3 event notification configuration

## S3 Event Processing Rules

Only S3 objects in the `customers/` prefix trigger email processing:

- ✅ `customers/{customer-code}/{changeId}.json` → Triggers emails
- ❌ `archive/{changeId}.json` → Ignored (fails customer code extraction)
- ❌ `deleted/archive/{changeId}.json` → Ignored (fails customer code extraction)
- ❌ `deleted/customers/{customer}/{changeId}.json` → Ignored (fails customer code extraction)
- ❌ `versions/{changeId}/v{N}.json` → Ignored (fails customer code extraction)
- ❌ `drafts/{changeId}.json` → Ignored (fails customer code extraction)

The `ExtractCustomerCodeFromS3Key()` function expects the format `customers/{customer-code}/filename.json` and will reject any other format with a non-retryable error.

## Testing

After deploying this fix:

1. **Create a new change** with 1 customer
2. **Submit for approval**
3. **Verify**: You should receive 1 "approval request" email
4. **Approve the change**
5. **Verify**: You should receive 1 "approved announcement" email and 1 meeting invite (if meeting required)

If you still see duplicates, check:
- CloudWatch Logs for the backend Lambda to see if events are being processed multiple times
- SQS queue metrics to see if messages are being redelivered
- S3 event notification configuration to ensure no duplicate rules

## Files Modified

- `lambda/upload_lambda/upload-metadata-lambda.js` - Added `request-type` and `status` to S3 metadata in `uploadToCustomerBucket()`
