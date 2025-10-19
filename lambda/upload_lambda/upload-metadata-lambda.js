import AWS from 'aws-sdk';
import { DateTime, parseDateTime, toRFC3339, toLogFormat, validateDateTime, validateMeetingTime, ERROR_TYPES } from './datetime/index.js';

// Initialize AWS services
const s3 = new AWS.S3();
const sqs = new AWS.SQS();

// Initialize datetime utilities with default config
const dateTime = new DateTime();

export const handler = async (event) => {
    try {
        console.log('📥 Request received:', {
            method: event.httpMethod || event.requestContext?.http?.method,
            path: event.path || event.resource || event.rawPath,
            headers: Object.keys(event.headers || {}),
            rawHeaders: event.headers
        });

        // Validate authentication headers added by Lambda@Edge SAML function
        // Headers can be lowercase or mixed case depending on the integration
        const headers = event.headers || {};
        const userEmail = headers['x-user-email'] || headers['X-User-Email'];
        const isAuthenticated = (headers['x-authenticated'] || headers['X-Authenticated']) === 'true';

        console.log('🔐 Auth check:', { userEmail, isAuthenticated });

        if (!isAuthenticated || !userEmail) {
            console.log('❌ Authentication failed - missing headers');
            return {
                statusCode: 401,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*',
                    'Access-Control-Allow-Headers': 'Content-Type',
                    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS'
                },
                body: JSON.stringify({ error: 'Authentication required - please log in through the web interface' })
            };
        }

        console.log('✅ User authenticated:', userEmail);

        // Validate user authorization
        if (!isAuthorizedForChangeManagement(userEmail)) {
            return {
                statusCode: 403,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Insufficient permissions for change management' })
            };
        }

        // Handle CORS preflight
        if (event.httpMethod === 'OPTIONS') {
            return {
                statusCode: 200,
                headers: {
                    'Access-Control-Allow-Origin': '*',
                    'Access-Control-Allow-Headers': 'Content-Type, x-user-email, x-authenticated',
                    'Access-Control-Allow-Methods': 'GET, POST, OPTIONS'
                },
                body: ''
            };
        }

        // Route based on path and method - handle both API Gateway and Lambda Function URL formats
        const path = event.path || event.resource || event.rawPath;
        const method = event.httpMethod || event.requestContext?.http?.method;

        // Route to appropriate handler
        if (path === '/upload' && method === 'POST') {
            return await handleUpload(event, userEmail);
        } else if (path === '/auth-check' && method === 'GET') {
            return await handleAuthCheck(event, userEmail);
        } else if (path === '/api/user' && method === 'GET') {
            // Simple endpoint to return current user email
            return {
                statusCode: 200,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({
                    email: userEmail,
                    user: userEmail,
                    authenticated: true
                })
            };
        } else if (path === '/api/user/context' && method === 'GET') {
            return await handleGetUserContext(event, userEmail);
        } else if (path === '/announcements' && method === 'GET') {
            console.log('📢 Routing to handleGetAnnouncements');
            return await handleGetAnnouncements(event, userEmail);
        } else if (path.startsWith('/announcements/customer/') && method === 'GET') {
            console.log('📢 Routing to handleGetCustomerAnnouncements');
            return await handleGetCustomerAnnouncements(event, userEmail);
        } else if (path.startsWith('/announcements/') && method === 'GET') {
            console.log('📢 Routing to handleGetAnnouncement');
            return await handleGetAnnouncement(event, userEmail);
        } else if (path.startsWith('/announcements/') && method === 'DELETE') {
            console.log('📢 Routing to handleDeleteAnnouncement');
            return await handleDeleteAnnouncement(event, userEmail);
        } else if (path === '/changes' && method === 'GET') {
            return await handleGetChanges(event, userEmail);
        } else if (path.startsWith('/changes/') && path.includes('/versions') && method === 'GET') {
            return await handleGetChangeVersions(event, userEmail);
        } else if (path.startsWith('/changes/') && method === 'GET') {
            return await handleGetChange(event, userEmail);
        } else if (path.startsWith('/changes/') && path.includes('/approve') && method === 'POST') {
            return await handleApproveChange(event, userEmail);
        } else if (path.startsWith('/changes/') && path.includes('/complete') && method === 'POST') {
            return await handleCompleteChange(event, userEmail);
        } else if (path.startsWith('/changes/') && path.includes('/cancel') && method === 'POST') {
            return await handleCancelChange(event, userEmail);
        } else if (path.startsWith('/changes/') && method === 'PUT') {
            return await handleUpdateChange(event, userEmail);
        } else if (path === '/my-changes' && method === 'GET') {
            return await handleGetMyChanges(event, userEmail);
        } else if (path === '/drafts' && method === 'GET') {
            return await handleGetDrafts(event, userEmail);
        } else if (path.startsWith('/drafts/') && method === 'GET') {
            return await handleGetDraft(event, userEmail);
        } else if (path === '/drafts' && method === 'POST') {
            return await handleSaveDraft(event, userEmail);
        } else if (path.startsWith('/drafts/') && method === 'DELETE') {
            return await handleDeleteDraft(event, userEmail);
        } else if (path.startsWith('/changes/') && method === 'DELETE') {
            return await handleDeleteChange(event, userEmail);
        } else if (path === '/changes/search' && method === 'POST') {
            return await handleSearchChanges(event, userEmail);
        } else if (path === '/changes/statistics' && method === 'GET') {
            return await handleGetStatistics(event, userEmail);
        } else if (path === '/changes/recent' && method === 'GET') {
            return await handleGetRecentChanges(event, userEmail);
        } else {
            return {
                statusCode: 404,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Endpoint not found' })
            };
        }

    } catch (error) {
        console.error('Error processing request:', error);

        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                error: 'Internal server error',
                message: error.message
            })
        };
    }
};

// Original upload handler - supports both changes and announcements
async function handleUpload(event, userEmail) {
    const metadata = JSON.parse(event.body);

    // Check if this is an announcement update action
    if (metadata.action === 'update_announcement') {
        return await handleUpdateAnnouncement(metadata, userEmail);
    }

    // Determine if this is a change or announcement based on object_type
    const isAnnouncement = metadata.object_type && metadata.object_type.startsWith('announcement_');

    // Validate required fields based on type
    if (isAnnouncement) {
        // Announcement validation
        if (!metadata.title || !metadata.customers || metadata.customers.length === 0) {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Missing required fields: title and customers' })
            };
        }
    } else {
        // Change validation
        if (!metadata.changeTitle || !metadata.customers || metadata.customers.length === 0) {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Missing required fields: changeTitle and customers' })
            };
        }
    }

    // Validate date/time fields if present
    try {
        let startDate = null;
        let endDate = null;

        if (metadata.implementationStart) {
            try {
                startDate = parseDateTime(metadata.implementationStart);
                validateDateTime(startDate);
                metadata.implementationStart = toRFC3339(startDate);
            } catch (error) {
                throw new Error(`implementationStart: ${error.message}`);
            }
        }

        if (metadata.implementationEnd) {
            try {
                endDate = parseDateTime(metadata.implementationEnd);
                validateDateTime(endDate);
                metadata.implementationEnd = toRFC3339(endDate);
            } catch (error) {
                throw new Error(`implementationEnd: ${error.message}`);
            }
        }

        // Validate date range if both dates are provided
        if (startDate && endDate) {
            dateTime.validateDateRange(startDate, endDate);
        }

        // Validate meeting times if present (support both meetingTime and meetingDate fields)
        const meetingTimeField = metadata.meetingTime || metadata.meetingDate;
        if (meetingTimeField) {
            try {
                const meetingDate = parseDateTime(meetingTimeField);
                validateMeetingTime(meetingDate);
                const rfc3339Time = toRFC3339(meetingDate);
                metadata.meetingTime = rfc3339Time;
                // Also set meetingDate for backward compatibility
                if (metadata.meetingDate) {
                    metadata.meetingDate = rfc3339Time;
                }
            } catch (error) {
                throw new Error(`meetingTime/meetingDate: ${error.message}`);
            }
        }
    } catch (error) {
        console.error('Date/time validation error:', error);
        return {
            statusCode: 400,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                error: 'Invalid date/time format',
                details: error.message,
                type: error.type || 'VALIDATION_ERROR'
            })
        };
    }

    // Add user context to metadata
    metadata.submittedBy = userEmail;
    metadata.submittedAt = toRFC3339(new Date());

    // Handle ID generation based on type
    if (isAnnouncement) {
        // Announcements should already have an ID from the frontend
        if (!metadata.announcement_id) {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Missing announcement_id' })
            };
        }
    } else {
        // Only generate new change ID if one doesn't exist (preserve draft IDs)
        if (!metadata.changeId) {
            metadata.changeId = generateChangeId();
        }
    }

    // Set status - announcements may already have a status from frontend
    if (!metadata.status) {
        metadata.status = 'submitted';
    }

    // Set prior_status for newly created objects
    if (!metadata.prior_status) {
        metadata.prior_status = '';
    }

    // Only set version and creation info if not already set (preserve draft info)
    if (!metadata.version) {
        metadata.version = 1;
    }
    if (!metadata.createdAt) {
        metadata.createdAt = metadata.submittedAt;
        metadata.createdBy = userEmail;
    }

    metadata.modifiedAt = metadata.submittedAt;
    metadata.modifiedBy = userEmail;

    // Upload to S3 for each customer
    const uploadResults = await uploadToCustomerBuckets(metadata);

    // Send SQS notifications
    await sendSQSNotifications(metadata, uploadResults);

    // After successful submission, delete the corresponding draft to prevent duplicates
    try {
        const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
        const draftKey = `drafts/${metadata.changeId}.json`;

        // Check if draft exists
        try {
            await s3.headObject({
                Bucket: bucketName,
                Key: draftKey
            }).promise();

            // Draft exists, move it to deleted folder
            const draftData = await s3.getObject({
                Bucket: bucketName,
                Key: draftKey
            }).promise();

            const draft = JSON.parse(draftData.Body.toString());

            // Add submission metadata to the draft before moving to deleted
            draft.submittedAt = metadata.submittedAt;
            draft.submittedBy = metadata.submittedBy;
            draft.deletedAt = toRFC3339(new Date());
            draft.deletedBy = userEmail;
            draft.deletionReason = 'submitted';
            draft.originalPath = draftKey;

            // Move to deleted folder
            const deletedKey = `deleted/drafts/${metadata.changeId}.json`;
            await s3.putObject({
                Bucket: bucketName,
                Key: deletedKey,
                Body: JSON.stringify(draft, null, 2),
                ContentType: 'application/json',
                Metadata: {
                    'change-id': draft.changeId,
                    'deleted-by': userEmail,
                    'deleted-at': draft.deletedAt,
                    'deletion-reason': 'submitted',
                    'original-path': draftKey
                }
            }).promise();

            // Delete the original draft
            await s3.deleteObject({
                Bucket: bucketName,
                Key: draftKey
            }).promise();



        } catch (error) {
            if (error.code === 'NotFound' || error.code === 'NoSuchKey') {

            } else {
                console.error(`⚠️ Failed to clean up draft ${metadata.changeId}:`, error);
                // Don't fail the submission if draft cleanup fails
            }
        }
    } catch (error) {
        console.error('Error during draft cleanup:', error);
        // Don't fail the submission if draft cleanup fails
    }

    // Return results
    const successCount = uploadResults.filter(r => r.success).length;
    const failureCount = uploadResults.filter(r => !r.success).length;

    // Build response with appropriate ID field
    const responseBody = {
        success: true,
        uploadResults: uploadResults,
        summary: {
            total: uploadResults.length,
            successful: successCount,
            failed: failureCount
        }
    };

    // Add appropriate ID field based on type
    if (isAnnouncement) {
        responseBody.announcement_id = metadata.announcement_id;
    } else {
        responseBody.changeId = metadata.changeId;
    }

    return {
        statusCode: 200,
        headers: {
            'Content-Type': 'application/json',
            'Access-Control-Allow-Origin': '*'
        },
        body: JSON.stringify(responseBody)
    };
}

// Handle announcement status updates (approve, cancel, complete)
async function handleUpdateAnnouncement(payload, userEmail) {
    console.log('📢 Handling announcement update:', {
        announcement_id: payload.announcement_id,
        status: payload.status,
        user: userEmail
    });

    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const announcementId = payload.announcement_id;
    const newStatus = payload.status;
    const modification = payload.modification;

    // Validate required fields
    if (!announcementId) {
        return {
            statusCode: 400,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Missing announcement_id' })
        };
    }

    if (!newStatus) {
        return {
            statusCode: 400,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Missing status' })
        };
    }

    // Validate status transitions
    const validStatuses = ['draft', 'submitted', 'approved', 'cancelled', 'completed'];
    if (!validStatuses.includes(newStatus)) {
        return {
            statusCode: 400,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: `Invalid status: ${newStatus}` })
        };
    }

    try {
        // CRITICAL: Read announcement from drafts, archive, and customer paths
        // The Go backend may have updated the customer path with meeting metadata
        // We need to use whichever version has the most recent data
        const archiveKey = `archive/${announcementId}.json`;
        const draftKey = `drafts/${announcementId}.json`;
        let announcementData;
        let archiveData = null;
        let customerData = null;
        let draftData = null;

        // Try to read from drafts folder first (for draft -> submitted transitions)
        try {
            const data = await s3.getObject({
                Bucket: bucketName,
                Key: draftKey
            }).promise();
            draftData = JSON.parse(data.Body.toString());
            console.log(`📥 Read announcement from drafts: ${draftKey}`);
        } catch (error) {
            if (error.code !== 'NoSuchKey') {
                throw error;
            }
            console.log(`⚠️  Announcement not found in drafts: ${draftKey}`);
        }

        // Try to read from archive
        try {
            const data = await s3.getObject({
                Bucket: bucketName,
                Key: archiveKey
            }).promise();
            archiveData = JSON.parse(data.Body.toString());
            console.log(`📥 Read announcement from archive: ${archiveKey}`);
        } catch (error) {
            if (error.code !== 'NoSuchKey') {
                throw error;
            }
            console.log(`⚠️  Announcement not found in archive: ${archiveKey}`);
        }

        // Try to read from customer path (if customers specified)
        const customers = payload.customers || (archiveData && archiveData.customers) || (draftData && draftData.customers) || [];
        if (customers.length > 0) {
            const customerKey = `customers/${customers[0]}/${announcementId}.json`;
            try {
                const data = await s3.getObject({
                    Bucket: bucketName,
                    Key: customerKey
                }).promise();
                customerData = JSON.parse(data.Body.toString());
                console.log(`📥 Read announcement from customer path: ${customerKey}`);
            } catch (error) {
                if (error.code !== 'NoSuchKey') {
                    throw error;
                }
                console.log(`⚠️  Announcement not found in customer path: ${customerKey}`);
            }
        }

        // Use whichever version has meeting metadata (most recent)
        // Customer path is updated by Go backend with meeting info
        if (customerData && customerData.meeting_metadata) {
            console.log('✅ Using customer path data (has meeting metadata)');
            announcementData = customerData;
        } else if (archiveData) {
            console.log('✅ Using archive path data');
            announcementData = archiveData;
        } else if (customerData) {
            console.log('✅ Using customer path data');
            announcementData = customerData;
        } else if (draftData) {
            console.log('✅ Using draft data');
            announcementData = draftData;
        } else {
            return {
                statusCode: 404,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Announcement not found in drafts, archive, or customer paths' })
            };
        }

        // Validate status transition
        const currentStatus = announcementData.status;
        const validTransitions = {
            'draft': ['submitted', 'cancelled'],
            'submitted': ['approved', 'cancelled'],
            'approved': ['completed', 'cancelled'],
            'completed': [],
            'cancelled': []
        };

        const allowedTransitions = validTransitions[currentStatus] || [];
        if (!allowedTransitions.includes(newStatus)) {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({
                    error: `Invalid status transition from ${currentStatus} to ${newStatus}`,
                    currentStatus,
                    requestedStatus: newStatus,
                    allowedTransitions
                })
            };
        }

        // Update announcement status with prior_status tracking
        announcementData.prior_status = announcementData.status;
        announcementData.status = newStatus;
        announcementData.modifiedAt = toRFC3339(new Date());
        announcementData.modifiedBy = userEmail;

        // Add modification entry
        if (!announcementData.modifications) {
            announcementData.modifications = [];
        }

        if (modification) {
            announcementData.modifications.push(modification);
        } else {
            // Create default modification entry
            announcementData.modifications.push({
                timestamp: announcementData.modifiedAt,
                user_id: userEmail,
                modification_type: newStatus
            });
        }

        // Get customers list (use provided list or existing list from loaded data)
        const finalCustomers = payload.customers || announcementData.customers || [];

        if (finalCustomers.length === 0) {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'No customers specified for announcement' })
            };
        }

        // Update announcement in archive
        await s3.putObject({
            Bucket: bucketName,
            Key: archiveKey,
            Body: JSON.stringify(announcementData, null, 2),
            ContentType: 'application/json'
        }).promise();

        console.log(`✅ Updated announcement in archive: ${archiveKey}`);

        // If transitioning from draft to submitted, delete the draft
        if (announcementData.status === 'submitted' && draftData) {
            try {
                const draftKey = `drafts/${announcementId}.json`;
                await s3.deleteObject({
                    Bucket: bucketName,
                    Key: draftKey
                }).promise();
                console.log(`✅ Deleted draft after submission: ${draftKey}`);
            } catch (error) {
                console.error(`⚠️ Failed to delete draft ${announcementId}:`, error);
                // Don't fail the submission if draft cleanup fails
            }
        }

        // Update announcement for each customer
        const updateResults = [];

        for (const customer of finalCustomers) {
            const customerKey = `customers/${customer}/${announcementId}.json`;

            try {
                // Determine request-type based on new status for proper backend routing
                let requestType = 'announcement_update';
                if (newStatus === 'approved') {
                    requestType = 'approved_announcement';
                } else if (newStatus === 'cancelled') {
                    requestType = 'announcement_cancelled';
                } else if (newStatus === 'completed') {
                    requestType = 'announcement_completed';
                } else if (newStatus === 'submitted') {
                    requestType = 'announcement_approval_request';
                }

                await s3.putObject({
                    Bucket: bucketName,
                    Key: customerKey,
                    Body: JSON.stringify(announcementData, null, 2),
                    ContentType: 'application/json',
                    Metadata: {
                        'announcement-id': announcementId,
                        'customer-code': customer,
                        'status': newStatus,
                        'modified-by': userEmail,
                        'modified-at': announcementData.modifiedAt,
                        'object-type': announcementData.object_type,
                        'request-type': requestType  // Tell backend what type of notification to send
                    }
                }).promise();

                console.log(`✅ Updated announcement for customer ${customer}: ${customerKey}`);

                updateResults.push({
                    customer,
                    success: true,
                    key: customerKey
                });
            } catch (error) {
                console.error(`❌ Failed to update announcement for customer ${customer}:`, error);
                updateResults.push({
                    customer,
                    success: false,
                    error: error.message,
                    key: customerKey
                });
            }
        }

        // Send SQS notification if status is approved (triggers backend processing)
        if (newStatus === 'approved') {
            try {
                await sendSQSNotifications(announcementData, updateResults);
                console.log('✅ Sent SQS notification for approved announcement');
            } catch (error) {
                console.error('⚠️ Failed to send SQS notification:', error);
                // Don't fail the update if SQS notification fails
            }
        }

        // Return success response
        const successCount = updateResults.filter(r => r.success).length;
        const failureCount = updateResults.filter(r => !r.success).length;

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                success: true,
                announcement_id: announcementId,
                status: newStatus,
                updateResults,
                summary: {
                    total: updateResults.length,
                    successful: successCount,
                    failed: failureCount
                }
            })
        };

    } catch (error) {
        console.error('Error updating announcement:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                error: 'Failed to update announcement',
                message: error.message
            })
        };
    }
}

// Get all changes (for view-changes page) - filters by CHG- prefix
async function handleGetChanges(event, userEmail) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const prefix = 'archive/';

    try {
        const params = {
            Bucket: bucketName,
            Prefix: prefix,
            MaxKeys: 1000
        };

        const result = await s3.listObjectsV2(params).promise();
        const changes = [];

        // Get metadata for each change file and filter by CHG- prefix
        for (const object of result.Contents) {
            try {
                const getParams = {
                    Bucket: bucketName,
                    Key: object.Key
                };

                const data = await s3.getObject(getParams).promise();
                const change = JSON.parse(data.Body.toString());

                // ONLY include objects with changeId starting with CHG-
                // This excludes announcements (CIC-, FIN-, INN-, GEN-) and any other object types
                if (!change.changeId || !change.changeId.startsWith('CHG-')) {
                    continue; // Skip non-change objects
                }

                // Add S3 metadata
                change.lastModified = object.LastModified;
                change.size = object.Size;

                changes.push(change);
            } catch (error) {
                console.error(`Error reading change file ${object.Key}:`, error);
            }
        }

        // Group by changeId and keep only the latest version of each change
        const changeMap = new Map();

        changes.forEach(change => {
            const existingChange = changeMap.get(change.changeId);
            if (!existingChange || (change.version || 1) > (existingChange.version || 1)) {
                changeMap.set(change.changeId, change);
            }
        });

        // Convert back to array and sort by modified date (newest first)
        const latestChanges = Array.from(changeMap.values());
        latestChanges.sort((a, b) => {
            const dateA = parseDateTime(b.modifiedAt);
            const dateB = parseDateTime(a.modifiedAt);
            return dateA.getTime() - dateB.getTime();
        });

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify(latestChanges)
        };

    } catch (error) {
        console.error('Error getting changes:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to retrieve changes' })
        };
    }
}

// Delete announcement (draft or cancelled only, per state machine)
async function handleDeleteAnnouncement(event, userEmail) {
    const announcementId = event.pathParameters?.announcementId || (event.path || event.rawPath).split('/').pop();
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `archive/${announcementId}.json`;

    try {
        // First verify the announcement exists and user owns it
        let announcement;
        try {
            const data = await s3.getObject({
                Bucket: bucketName,
                Key: key
            }).promise();

            announcement = JSON.parse(data.Body.toString());
            console.log('📋 Loaded announcement from S3 for deletion');
            console.log('📋 Announcement status:', announcement.status);

            // Verify ownership
            if (announcement.created_by !== userEmail && announcement.submittedBy !== userEmail) {
                return {
                    statusCode: 403,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Access denied to delete this announcement' })
                };
            }
        } catch (error) {
            if (error.code === 'NoSuchKey') {
                return {
                    statusCode: 404,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Announcement not found' })
                };
            }
            throw error;
        }

        // Validate status - can only delete draft or cancelled (per state machine)
        if (announcement.status !== 'draft' && announcement.status !== 'cancelled') {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({
                    error: `Cannot delete announcement with status: ${announcement.status}. Only draft or cancelled announcements can be deleted.`,
                    currentStatus: announcement.status
                })
            };
        }

        // Move the announcement to deleted folder instead of permanently deleting
        const deletedKey = `deleted/archive/${announcementId}.json`;

        // Add deletion metadata
        announcement.deletedAt = toRFC3339(new Date());
        announcement.deletedBy = userEmail;
        announcement.originalPath = key;

        // Copy to deleted folder
        await s3.putObject({
            Bucket: bucketName,
            Key: deletedKey,
            Body: JSON.stringify(announcement, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'announcement-id': announcement.announcement_id,
                'deleted-by': userEmail,
                'deleted-at': announcement.deletedAt,
                'original-path': key
            }
        }).promise();

        console.log(`✅ Copied announcement to deleted folder: ${deletedKey}`);

        // Delete from archive
        await s3.deleteObject({
            Bucket: bucketName,
            Key: key
        }).promise();

        console.log(`✅ Deleted announcement from archive: ${key}`);

        // Also delete from customer buckets
        const customers = announcement.customers || [];
        for (const customerCode of customers) {
            const customerKey = `customers/${customerCode}/${announcementId}.json`;
            try {
                await s3.deleteObject({
                    Bucket: bucketName,
                    Key: customerKey
                }).promise();
                console.log(`✅ Deleted announcement from customer bucket: ${customerKey}`);
            } catch (error) {
                console.warn(`⚠️  Failed to delete from customer bucket ${customerKey}:`, error.message);
            }
        }

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                message: 'Announcement deleted successfully',
                announcementId: announcementId,
                deletedPath: deletedKey
            })
        };

    } catch (error) {
        console.error('Error deleting announcement:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to delete announcement' })
        };
    }
}

// Get all announcements - filters by CIC-, FIN-, INN- prefixes
async function handleGetAnnouncements(event, userEmail) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const prefix = 'archive/';

    try {
        const params = {
            Bucket: bucketName,
            Prefix: prefix,
            MaxKeys: 1000
        };

        const result = await s3.listObjectsV2(params).promise();
        const announcements = [];

        // Get metadata for each file and filter by announcement prefixes
        for (const object of result.Contents) {
            try {
                const getParams = {
                    Bucket: bucketName,
                    Key: object.Key
                };

                const data = await s3.getObject(getParams).promise();
                const announcement = JSON.parse(data.Body.toString());

                // ONLY include objects with announcement_id starting with CIC-, FIN-, INN-, or GEN-
                if (!announcement.announcement_id ||
                    !(announcement.announcement_id.startsWith('CIC-') ||
                        announcement.announcement_id.startsWith('FIN-') ||
                        announcement.announcement_id.startsWith('INN-') ||
                        announcement.announcement_id.startsWith('GEN-'))) {
                    continue;
                }

                // Add S3 metadata
                announcement.lastModified = object.LastModified;
                announcement.size = object.Size;

                announcements.push(announcement);
            } catch (error) {
                console.error(`Error reading announcement file ${object.Key}:`, error);
            }
        }

        // Sort by posted_date or created_at (newest first)
        announcements.sort((a, b) => {
            try {
                const dateStrA = b.posted_date || b.created_at || b.lastModified;
                const dateStrB = a.posted_date || a.created_at || a.lastModified;

                if (!dateStrA && !dateStrB) return 0;
                if (!dateStrA) return 1;
                if (!dateStrB) return -1;

                const dateA = parseDateTime(dateStrA);
                const dateB = parseDateTime(dateStrB);
                return dateA.getTime() - dateB.getTime();
            } catch (error) {
                // If date parsing fails, fall back to lastModified comparison
                const fallbackA = b.lastModified ? new Date(b.lastModified).getTime() : 0;
                const fallbackB = a.lastModified ? new Date(a.lastModified).getTime() : 0;
                return fallbackA - fallbackB;
            }
        });

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify(announcements)
        };

    } catch (error) {
        console.error('Error getting announcements:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to retrieve announcements' })
        };
    }
}

// Get announcements for a specific customer
async function handleGetCustomerAnnouncements(event, userEmail) {
    const customerCode = event.pathParameters?.customerCode || (event.path || event.rawPath).split('/').pop();
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const prefix = 'archive/';

    try {
        const params = {
            Bucket: bucketName,
            Prefix: prefix,
            MaxKeys: 1000
        };

        const result = await s3.listObjectsV2(params).promise();
        const announcements = [];

        // Get metadata for each file and filter by customer and announcement prefixes
        for (const object of result.Contents) {
            try {
                const getParams = {
                    Bucket: bucketName,
                    Key: object.Key
                };

                const data = await s3.getObject(getParams).promise();
                const announcement = JSON.parse(data.Body.toString());

                // ONLY include objects with announcement_id starting with CIC-, FIN-, INN-, or GEN-
                if (!announcement.announcement_id ||
                    !(announcement.announcement_id.startsWith('CIC-') ||
                        announcement.announcement_id.startsWith('FIN-') ||
                        announcement.announcement_id.startsWith('INN-') ||
                        announcement.announcement_id.startsWith('GEN-'))) {
                    continue;
                }

                // Filter: must be for this customer
                const isForCustomer = (Array.isArray(announcement.customers) && announcement.customers.includes(customerCode)) ||
                    (announcement.customer === customerCode);

                if (!isForCustomer) {
                    continue;
                }

                // Add S3 metadata
                announcement.lastModified = object.LastModified;
                announcement.size = object.Size;

                announcements.push(announcement);
            } catch (error) {
                console.error(`Error reading announcement file ${object.Key}:`, error);
            }
        }

        // Sort by posted_date or created_at (newest first)
        announcements.sort((a, b) => {
            try {
                const dateStrA = b.posted_date || b.created_at || b.lastModified;
                const dateStrB = a.posted_date || a.created_at || a.lastModified;

                if (!dateStrA && !dateStrB) return 0;
                if (!dateStrA) return 1;
                if (!dateStrB) return -1;

                const dateA = parseDateTime(dateStrA);
                const dateB = parseDateTime(dateStrB);
                return dateA.getTime() - dateB.getTime();
            } catch (error) {
                // If date parsing fails, fall back to lastModified comparison
                const fallbackA = b.lastModified ? new Date(b.lastModified).getTime() : 0;
                const fallbackB = a.lastModified ? new Date(a.lastModified).getTime() : 0;
                return fallbackA - fallbackB;
            }
        });

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify(announcements)
        };

    } catch (error) {
        console.error('Error getting customer announcements:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to retrieve customer announcements' })
        };
    }
}

// Get specific change by ID
async function handleGetChange(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').pop();
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `archive/${changeId}.json`;

    try {
        const params = {
            Bucket: bucketName,
            Key: key
        };

        const data = await s3.getObject(params).promise();
        const change = JSON.parse(data.Body.toString());

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify(change)
        };

    } catch (error) {
        if (error.code === 'NoSuchKey') {
            return {
                statusCode: 404,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Change not found' })
            };
        }

        console.error('Error getting change:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to retrieve change' })
        };
    }
}

// Get changes for current user (for my-changes page)
async function handleGetMyChanges(event, userEmail) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const prefix = 'archive/';

    try {
        const params = {
            Bucket: bucketName,
            Prefix: prefix,
            MaxKeys: 1000
        };

        const result = await s3.listObjectsV2(params).promise();
        const myChanges = [];

        // Get metadata for each change file and filter by user
        for (const object of result.Contents) {
            try {
                const getParams = {
                    Bucket: bucketName,
                    Key: object.Key
                };

                const data = await s3.getObject(getParams).promise();
                const change = JSON.parse(data.Body.toString());

                // ONLY include objects with changeId starting with CHG- AND created by this user
                if (change.changeId && change.changeId.startsWith('CHG-') &&
                    (change.createdBy === userEmail || change.submittedBy === userEmail)) {
                    change.lastModified = object.LastModified;
                    change.size = object.Size;
                    myChanges.push(change);
                }
            } catch (error) {
                console.error(`Error reading change file ${object.Key}:`, error);
            }
        }

        // Group by changeId and keep only the latest version of each change
        const changeMap = new Map();

        myChanges.forEach(change => {
            const existingChange = changeMap.get(change.changeId);
            if (!existingChange || (change.version || 1) > (existingChange.version || 1)) {
                changeMap.set(change.changeId, change);
            }
        });

        // Convert back to array and sort by modified date (newest first)
        const latestChanges = Array.from(changeMap.values());
        latestChanges.sort((a, b) => {
            const dateA = parseDateTime(b.modifiedAt);
            const dateB = parseDateTime(a.modifiedAt);
            return dateA.getTime() - dateB.getTime();
        });

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify(latestChanges)
        };

    } catch (error) {
        console.error('Error getting my changes:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to retrieve your changes' })
        };
    }
}

// Get drafts for current user
async function handleGetDrafts(event, userEmail) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const prefix = 'drafts/';

    try {
        const params = {
            Bucket: bucketName,
            Prefix: prefix,
            MaxKeys: 1000
        };

        const result = await s3.listObjectsV2(params).promise();
        const myDrafts = [];

        // Get metadata for each draft file and filter by user
        for (const object of result.Contents) {
            try {
                const getParams = {
                    Bucket: bucketName,
                    Key: object.Key
                };

                const data = await s3.getObject(getParams).promise();
                const draft = JSON.parse(data.Body.toString());

                // Only include drafts created by this user
                if (draft.createdBy === userEmail || draft.submittedBy === userEmail) {
                    draft.lastModified = object.LastModified;
                    draft.size = object.Size;

                    // Ensure changeId is set for changes (frontend expects this field)
                    // Announcements use announcement_id
                    const isAnnouncement = draft.announcement_id || draft.object_type?.startsWith('announcement_');
                    if (!isAnnouncement && !draft.changeId) {
                        // Extract ID from S3 key (drafts/SOME-ID.json -> SOME-ID)
                        const keyParts = object.Key.split('/');
                        const filename = keyParts[keyParts.length - 1];
                        const idFromKey = filename.replace('.json', '');
                        draft.changeId = idFromKey;
                        console.log(`⚠️  Draft missing changeId, extracted from key: ${idFromKey}`);
                    }

                    myDrafts.push(draft);
                }
            } catch (error) {
                console.error(`Error reading draft file ${object.Key}:`, error);
            }
        }

        // Sort by modified date (newest first)
        myDrafts.sort((a, b) => {
            const dateA = parseDateTime(b.modifiedAt);
            const dateB = parseDateTime(a.modifiedAt);
            return dateA.getTime() - dateB.getTime();
        });

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify(myDrafts)
        };

    } catch (error) {
        console.error('Error getting drafts:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to retrieve drafts' })
        };
    }
}

// Get specific draft by ID
async function handleGetDraft(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').pop();
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `drafts/${changeId}.json`;

    try {
        const params = {
            Bucket: bucketName,
            Key: key
        };

        const data = await s3.getObject(params).promise();
        const draft = JSON.parse(data.Body.toString());

        // Verify user owns this draft
        if (draft.createdBy !== userEmail && draft.submittedBy !== userEmail) {
            return {
                statusCode: 403,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Access denied to this draft' })
            };
        }

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify(draft)
        };

    } catch (error) {
        if (error.code === 'NoSuchKey') {
            return {
                statusCode: 404,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Draft not found' })
            };
        }

        console.error('Error getting draft:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to retrieve draft' })
        };
    }
}

// Save draft
async function handleSaveDraft(event, userEmail) {
    const draft = JSON.parse(event.body);

    // Determine if this is a change or announcement
    const isAnnouncement = draft.announcement_id || draft.object_type?.startsWith('announcement_');
    const objectId = isAnnouncement ? draft.announcement_id : draft.changeId;
    const objectType = isAnnouncement ? 'announcement' : 'change';

    console.log(`📝 Saving draft ${objectType}:`, objectId);
    console.log(`📋 Is announcement:`, isAnnouncement);
    console.log(`📋 Object ID:`, objectId);

    // Validate required fields
    if (!objectId) {
        console.error(`❌ Missing required field for ${objectType}`);
        return {
            statusCode: 400,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: `Missing required field: ${isAnnouncement ? 'announcement_id' : 'changeId'}` })
        };
    }

    // Validate and normalize date/time fields if present (changes only)
    if (!isAnnouncement) {
        try {
            let startDate = null;
            let endDate = null;

            if (draft.implementationStart) {
                startDate = parseDateTime(draft.implementationStart);
                validateDateTime(startDate);
                draft.implementationStart = toRFC3339(startDate);
            }

            if (draft.implementationEnd) {
                endDate = parseDateTime(draft.implementationEnd);
                validateDateTime(endDate);
                draft.implementationEnd = toRFC3339(endDate);
            }

            // Validate date range if both dates are provided
            if (startDate && endDate) {
                dateTime.validateDateRange(startDate, endDate);
            }

            // Validate meeting times if present
            if (draft.meetingTime) {
                const meetingDate = parseDateTime(draft.meetingTime);
                validateMeetingTime(meetingDate);
                draft.meetingTime = toRFC3339(meetingDate);
            }
        } catch (error) {
            console.error('Date/time validation error in draft:', error);
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({
                    error: 'Invalid date/time format in draft',
                    details: error.message,
                    type: error.type || 'VALIDATION_ERROR'
                })
            };
        }
    }

    // Add/update user context
    draft.status = 'draft';
    draft.modifiedAt = toRFC3339(new Date());
    draft.modifiedBy = userEmail;

    if (!draft.createdAt) {
        draft.createdAt = draft.modifiedAt;
        draft.createdBy = userEmail;
    }

    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `drafts/${objectId}.json`;

    try {
        const metadata = {
            'created-by': draft.createdBy || draft.created_by,
            'modified-by': draft.modifiedBy || draft.modified_by,
            'status': 'draft'
        };

        // Add appropriate ID field to metadata
        if (isAnnouncement) {
            metadata['announcement-id'] = objectId;
        } else {
            metadata['change-id'] = objectId;
        }

        const params = {
            Bucket: bucketName,
            Key: key,
            Body: JSON.stringify(draft, null, 2),
            ContentType: 'application/json',
            Metadata: metadata
        };

        await s3.putObject(params).promise();

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                success: true,
                changeId: draft.changeId,
                message: 'Draft saved successfully'
            })
        };

    } catch (error) {
        console.error('Error saving draft:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to save draft' })
        };
    }
}

// Search changes
async function handleSearchChanges(event, userEmail) {
    const searchCriteria = JSON.parse(event.body);
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';

    try {
        let changes = [];

        // Determine which folders to search based on status filter
        let foldersToSearch = [];

        if (!searchCriteria.status || searchCriteria.status === '' || searchCriteria.status === 'all') {
            // No status filter or "All Statuses" - search both folders
            foldersToSearch = ['archive/', 'drafts/'];
        } else if (searchCriteria.status === 'draft') {
            // Draft status - search only drafts folder
            foldersToSearch = ['drafts/'];
        } else {
            // Submitted, approved, completed, etc. - search only archive folder
            foldersToSearch = ['archive/'];
        }

        for (const prefix of foldersToSearch) {
            const params = {
                Bucket: bucketName,
                Prefix: prefix,
                MaxKeys: 1000
            };

            const result = await s3.listObjectsV2(params).promise();

            // Get metadata for each change file
            for (const object of result.Contents) {
                try {
                    const getParams = {
                        Bucket: bucketName,
                        Key: object.Key
                    };

                    const data = await s3.getObject(getParams).promise();
                    const change = JSON.parse(data.Body.toString());

                    // Only include changes created by this user for drafts
                    if (prefix === 'drafts/' && change.createdBy !== userEmail && change.submittedBy !== userEmail) {
                        continue;
                    }

                    change.lastModified = object.LastModified;
                    change.size = object.Size;

                    changes.push(change);
                } catch (error) {
                    console.error(`Error reading change file ${object.Key}:`, error);
                }
            }
        }

        // Apply search filters
        changes = applySearchFilters(changes, searchCriteria);

        // Sort by relevance/date
        changes.sort((a, b) => {
            const dateA = parseDateTime(b.modifiedAt);
            const dateB = parseDateTime(a.modifiedAt);
            return dateA.getTime() - dateB.getTime();
        });

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify(changes)
        };

    } catch (error) {
        console.error('Error searching changes:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Search failed' })
        };
    }
}

function applySearchFilters(changes, criteria) {
    return changes.filter(change => {
        // Text search
        if (criteria.query) {
            const query = criteria.query.toLowerCase();
            const searchableText = [
                change.changeTitle || '',
                change.changeReason || '',
                change.implementationPlan || '',
                change.changeId || '',
                change.jiraTicket || '',
                change.snowTicket || ''
            ].join(' ').toLowerCase();

            if (!searchableText.includes(query)) {
                return false;
            }
        }

        // Status filter
        if (criteria.status && change.status !== criteria.status) {
            return false;
        }

        // Created by filter
        if (criteria.createdBy) {
            const createdBy = criteria.createdBy.toLowerCase();
            if (!(change.createdBy || '').toLowerCase().includes(createdBy)) {
                return false;
            }
        }

        // Customer filter
        if (criteria.customers && criteria.customers.length > 0) {
            const changeCustomers = change.customers || [];
            if (!criteria.customers.some(customer => changeCustomers.includes(customer))) {
                return false;
            }
        }

        // Date range filter
        if (criteria.startDate || criteria.endDate) {
            try {
                const changeDate = parseDateTime(change.modifiedAt);

                if (criteria.startDate) {
                    const startDate = parseDateTime(criteria.startDate);
                    if (changeDate < startDate) {
                        return false;
                    }
                }

                if (criteria.endDate) {
                    const endDate = parseDateTime(criteria.endDate);
                    endDate.setHours(23, 59, 59, 999); // End of day
                    if (changeDate > endDate) {
                        return false;
                    }
                }
            } catch (error) {
                console.error('Error parsing dates in search filter:', error);
                return false;
            }
        }

        return true;
    });
}

// Utility functions (from original Lambda)
function isAuthorizedForChangeManagement(userEmail) {
    if (!userEmail || !userEmail.endsWith('@hearst.com')) {
        return false;
    }

    return true;
}

async function uploadToCustomerBuckets(metadata, isUpdate = false, expectedETag = null) {
    // TRANSIENT TRIGGER PATTERN: Upload to archive FIRST to establish source of truth
    console.log('📦 Step 1: Uploading to archive (single source of truth)...');

    let archiveResult;
    try {
        if (isUpdate && expectedETag) {
            // Update with optimistic locking
            console.log('🔒 Using optimistic locking for update');
            archiveResult = await updateArchiveWithOptimisticLocking(metadata, 3);
        } else {
            // Initial creation
            archiveResult = await uploadToArchiveBucket(metadata, null);
        }
        console.log('✅ Archive upload successful:', archiveResult.key);
    } catch (error) {
        console.error('❌ Archive upload failed:', error);

        // Check if this is an ETag mismatch error
        if (error instanceof ETagMismatchError || error.message.includes('concurrent modifications')) {
            return [{
                customer: 'Archive (Permanent Storage)',
                success: false,
                error: error.message,
                errorType: 'CONCURRENT_MODIFICATION'
            }];
        }

        // If archive upload fails, don't create any customer triggers
        return [{
            customer: 'Archive (Permanent Storage)',
            success: false,
            error: error.message
        }];
    }

    // Step 2: Create transient triggers in customers/ prefix AFTER archive succeeds
    console.log('📦 Step 2: Creating transient triggers for customers...');
    const customerUploadPromises = [];

    for (const customer of metadata.customers) {
        customerUploadPromises.push(uploadToCustomerBucket(metadata, customer));
    }

    const customerResults = await Promise.allSettled(customerUploadPromises);

    // Build results array with archive first, then customers
    const results = [
        {
            customer: 'Archive (Permanent Storage)',
            success: true,
            s3Key: archiveResult.key,
            bucket: archiveResult.bucket
        }
    ];

    // Add customer results
    customerResults.forEach((result, index) => {
        const customerName = getCustomerDisplayName(metadata.customers[index]);

        if (result.status === 'fulfilled') {
            results.push({
                customer: customerName,
                success: true,
                s3Key: result.value.key,
                bucket: result.value.bucket
            });
            console.log(`✅ Trigger created for ${customerName}`);
        } else {
            results.push({
                customer: customerName,
                success: false,
                error: result.reason.message
            });
            console.error(`⚠️  Failed to create trigger for ${customerName}:`, result.reason.message);
            // Note: We log error but don't fail entire operation - archive is already saved
        }
    });

    console.log('✅ Upload sequence completed (archive-first pattern)');
    return results;
}

async function uploadToCustomerBucket(metadata, customer) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const isAnnouncement = metadata.object_type && metadata.object_type.startsWith('announcement_');
    const objectId = isAnnouncement ? metadata.announcement_id : metadata.changeId;
    const key = `customers/${customer}/${objectId}.json`;

    const s3Metadata = {
        'customer': customer,
        'submitted-by': metadata.submittedBy,
        'submitted-at': metadata.submittedAt,
        'status': metadata.status || 'submitted'
    };

    // Add type-specific metadata
    if (isAnnouncement) {
        s3Metadata['announcement-id'] = metadata.announcement_id;
        s3Metadata['object-type'] = metadata.object_type;
        s3Metadata['announcement-type'] = metadata.announcement_type;
        s3Metadata['request-type'] = 'announcement_approval_request';  // CRITICAL: Tell backend this is an announcement approval request
    } else {
        s3Metadata['change-id'] = metadata.changeId;
        s3Metadata['request-type'] = 'approval_request';  // CRITICAL: Tell backend this is an approval request
    }

    const params = {
        Bucket: bucketName,
        Key: key,
        Body: JSON.stringify(metadata, null, 2),
        ContentType: 'application/json',
        Metadata: s3Metadata
    };

    await s3.putObject(params).promise();
    return { bucket: bucketName, key: key };
}

async function uploadToArchiveBucket(metadata, expectedETag = null) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const isAnnouncement = metadata.object_type && metadata.object_type.startsWith('announcement_');
    const objectId = isAnnouncement ? metadata.announcement_id : metadata.changeId;
    const key = `archive/${objectId}.json`;

    const s3Metadata = {
        'submitted-by': metadata.submittedBy,
        'submitted-at': metadata.submittedAt,
        'customers': metadata.customers.join(',')
    };

    // Add type-specific metadata
    if (isAnnouncement) {
        s3Metadata['announcement-id'] = metadata.announcement_id;
        s3Metadata['object-type'] = metadata.object_type;
    } else {
        s3Metadata['change-id'] = metadata.changeId;
    }

    const params = {
        Bucket: bucketName,
        Key: key,
        Body: JSON.stringify(metadata, null, 2),
        ContentType: 'application/json',
        Metadata: s3Metadata
    };

    // OPTIMISTIC LOCKING: Add conditional write based on ETag
    if (expectedETag) {
        // Update existing object - use If-Match for optimistic locking
        params.IfMatch = expectedETag;
        console.log(`📝 Updating archive with ETag lock: ${expectedETag}`);
    } else {
        // Initial creation - use If-None-Match to prevent duplicates
        params.IfNoneMatch = '*';
        console.log(`📝 Creating new archive with duplicate prevention`);
    }

    try {
        await s3.putObject(params).promise();
        console.log(`✅ Archive upload successful with optimistic locking`);
        return { bucket: bucketName, key: key };
    } catch (error) {
        // Check for ETag mismatch (HTTP 412 Precondition Failed)
        if (error.code === 'PreconditionFailed' || error.statusCode === 412) {
            console.log(`⚠️  ETag mismatch detected - object was modified concurrently`);
            throw new ETagMismatchError(
                `Archive was modified by another user. Please refresh and try again.`,
                bucketName,
                key,
                expectedETag
            );
        }
        throw error;
    }
}

// Custom error class for ETag mismatches
class ETagMismatchError extends Error {
    constructor(message, bucket, key, expectedETag) {
        super(message);
        this.name = 'ETagMismatchError';
        this.bucket = bucket;
        this.key = key;
        this.expectedETag = expectedETag;
        this.statusCode = 409; // Conflict
    }
}

// Helper function to load archive with ETag
async function loadArchiveWithETag(objectId, isAnnouncement = false) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `archive/${objectId}.json`;

    try {
        const result = await s3.getObject({
            Bucket: bucketName,
            Key: key
        }).promise();

        const metadata = JSON.parse(result.Body.toString());
        const etag = result.ETag; // Capture ETag for optimistic locking

        console.log(`📥 Loaded archive with ETag: ${etag}`);
        return { metadata, etag };
    } catch (error) {
        if (error.code === 'NoSuchKey') {
            return { metadata: null, etag: null };
        }
        throw error;
    }
}

// Update archive with optimistic locking and retry
async function updateArchiveWithOptimisticLocking(metadata, maxRetries = 3) {
    const isAnnouncement = metadata.object_type && metadata.object_type.startsWith('announcement_');
    const objectId = isAnnouncement ? metadata.announcement_id : metadata.changeId;

    for (let attempt = 0; attempt <= maxRetries; attempt++) {
        try {
            if (attempt > 0) {
                // Exponential backoff: 100ms, 200ms, 400ms
                const delay = 100 * Math.pow(2, attempt - 1);
                console.log(`🔄 Retrying after ${delay}ms (attempt ${attempt + 1}/${maxRetries + 1})`);
                await new Promise(resolve => setTimeout(resolve, delay));
            }

            // Step 1: Load current state with ETag
            const { metadata: currentMetadata, etag } = await loadArchiveWithETag(objectId, isAnnouncement);

            if (!currentMetadata) {
                // Archive doesn't exist yet - create it
                return await uploadToArchiveBucket(metadata, null);
            }

            // Step 2: Merge changes (preserve existing modifications, add new ones)
            const mergedMetadata = {
                ...currentMetadata,
                ...metadata,
                modifications: [
                    ...(currentMetadata.modifications || []),
                    ...(metadata.modifications || [])
                ]
            };

            // Step 3: Write with ETag-based conditional update
            return await uploadToArchiveBucket(mergedMetadata, etag);

        } catch (error) {
            if (error instanceof ETagMismatchError) {
                if (attempt < maxRetries) {
                    console.log(`⚠️  ETag mismatch on attempt ${attempt + 1}, retrying...`);
                    continue; // Retry
                } else {
                    console.log(`❌ Failed after ${maxRetries + 1} attempts due to concurrent modifications`);
                    throw new Error(
                        `Unable to save changes after ${maxRetries + 1} attempts. ` +
                        `The change was modified by another user. Please refresh the page and try again.`
                    );
                }
            }
            throw error; // Other errors - don't retry
        }
    }
}

async function sendSQSNotifications(metadata, uploadResults) {
    const queueUrl = process.env.SQS_QUEUE_URL;

    if (!queueUrl) {
        return;
    }

    const successfulUploads = uploadResults.filter(r => r.success);

    for (const upload of successfulUploads) {
        if (upload.customer === 'Archive (Permanent Storage)') continue;

        const message = {
            changeId: metadata.changeId,
            customer: upload.customer,
            s3Bucket: upload.bucket,
            s3Key: upload.s3Key,
            submittedBy: metadata.submittedBy,
            submittedAt: metadata.submittedAt,
            changeTitle: metadata.changeTitle
        };

        try {
            await sqs.sendMessage({
                QueueUrl: queueUrl,
                MessageBody: JSON.stringify(message),
                MessageAttributes: {
                    'changeId': {
                        DataType: 'String',
                        StringValue: metadata.changeId
                    },
                    'customer': {
                        DataType: 'String',
                        StringValue: upload.customer
                    }
                }
            }).promise();


        } catch (error) {
            console.error(`Failed to send SQS notification for ${upload.customer}:`, error);
        }
    }
}

function generateChangeId() {
    const timestamp = toLogFormat(new Date()).replace(/[:.]/g, '-').slice(0, 19);
    const random = Math.random().toString(36).substring(2, 8);
    return `CHG-${timestamp}-${random}`;
}

function getCustomerDisplayName(customerCode) {
    const customerMapping = {
        'hts': 'HTS Prod',
        'htsnonprod': 'HTS NonProd',
        'cds': 'CDS Global',
        'fdbus': 'FDBUS',
        'hmiit': 'Hearst Magazines Italy',
        'hmies': 'Hearst Magazines Spain',
        'htvdigital': 'HTV Digital',
        'htv': 'HTV',
        'icx': 'iCrossing',
        'motor': 'Motor',
        'bat': 'Bring A Trailer',
        'mhk': 'MHK',
        'hdmautos': 'Autos',
        'hnpit': 'HNP IT',
        'hnpdigital': 'HNP Digital',
        'camp': 'CAMP Systems',
        'mcg': 'MCG',
        'hmuk': 'Hearst Magazines UK',
        'hmusdigital': 'Hearst Magazines Digital',
        'hwp': 'Hearst Western Properties',
        'zynx': 'Zynx',
        'hchb': 'HCHB',
        'fdbuk': 'FDBUK',
        'hecom': 'Hearst ECommerce',
        'blkbook': 'Black Book'
    };

    return customerMapping[customerCode] || customerCode;
}



// Get statistics for dashboard
async function handleGetStatistics(event, userEmail) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';

    console.log('📊 Getting statistics for user:', userEmail);

    try {

        // Get counts from different prefixes
        const [archiveObjects, draftObjects] = await Promise.all([
            s3.listObjectsV2({ Bucket: bucketName, Prefix: 'archive/' }).promise(),
            s3.listObjectsV2({ Bucket: bucketName, Prefix: 'drafts/' }).promise()
        ]);

        // Initialize counters
        let totalChanges = 0;
        let draftChanges = 0;
        let submittedChanges = 0;  // "Approval Requests"
        let approvedChanges = 0;
        let completedChanges = 0;
        let cancelledChanges = 0;

        // Count drafts (filter by user)
        if (draftObjects.Contents) {
            for (const obj of draftObjects.Contents) {
                try {
                    const objData = await s3.getObject({ Bucket: bucketName, Key: obj.Key }).promise();
                    const change = JSON.parse(objData.Body.toString());

                    // ONLY count objects with changeId starting with CHG-
                    if (!change.changeId || !change.changeId.startsWith('CHG-')) {
                        continue;
                    }

                    // Check if this change belongs to the current user (case-insensitive)
                    const changeCreatedBy = (change.createdBy || '').toLowerCase();
                    const changeSubmittedBy = (change.submittedBy || '').toLowerCase();
                    const changeModifiedBy = (change.modifiedBy || '').toLowerCase();
                    const currentUser = userEmail.toLowerCase();

                    if (changeCreatedBy === currentUser || changeSubmittedBy === currentUser || changeModifiedBy === currentUser) {
                        draftChanges++;
                    }
                } catch (error) {
                    console.warn(`Error reading draft ${obj.Key}:`, error.message);
                }
            }
        }

        // Count archive changes by actual status (filter by user)
        if (archiveObjects.Contents) {
            // Limit to recent files to avoid timeout (last 100 files)
            const recentObjects = archiveObjects.Contents
                .sort((a, b) => {
                    const dateA = new Date(b.LastModified);
                    const dateB = new Date(a.LastModified);
                    return dateA.getTime() - dateB.getTime();
                })
                .slice(0, 100);

            for (const obj of recentObjects) {
                try {
                    const objData = await s3.getObject({ Bucket: bucketName, Key: obj.Key }).promise();
                    const change = JSON.parse(objData.Body.toString());

                    // ONLY count objects with changeId starting with CHG-
                    if (!change.changeId || !change.changeId.startsWith('CHG-')) {
                        continue;
                    }

                    // Check if this change belongs to the current user (case-insensitive)
                    const changeCreatedBy = (change.createdBy || '').toLowerCase();
                    const changeSubmittedBy = (change.submittedBy || '').toLowerCase();
                    const changeModifiedBy = (change.modifiedBy || '').toLowerCase();
                    const currentUser = userEmail.toLowerCase();

                    const isUserChange = changeCreatedBy === currentUser ||
                        changeSubmittedBy === currentUser ||
                        changeModifiedBy === currentUser;

                    if (isUserChange) {
                        totalChanges++;

                        // Count by actual status
                        const status = change.status || 'submitted';
                        switch (status.toLowerCase()) {
                            case 'submitted':
                                submittedChanges++;
                                break;
                            case 'approved':
                                approvedChanges++;
                                break;
                            case 'completed':
                                completedChanges++;
                                break;
                            case 'cancelled':
                                cancelledChanges++;
                                break;
                            default:
                                // Treat unknown status as submitted
                                submittedChanges++;
                                break;
                        }
                    }
                } catch (error) {
                    console.warn(`Error reading archive ${obj.Key}:`, error.message);
                }
            }
        }

        // Add drafts to total count
        totalChanges += draftChanges;

        console.log('📊 Statistics calculated:', {
            userEmail,
            totalChanges,
            draftChanges,
            submittedChanges,
            approvedChanges,
            completedChanges,
            cancelledChanges
        });

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                total: totalChanges,
                draft: draftChanges,
                submitted: submittedChanges,  // This maps to "Approval Requests" in the UI
                approved: approvedChanges,
                completed: completedChanges,
                cancelled: cancelledChanges,
                active: submittedChanges
            })
        };

    } catch (error) {
        console.error('Error getting statistics:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to get statistics' })
        };
    }
}

// Get recent changes for dashboard
async function handleGetRecentChanges(event, userEmail) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const limit = parseInt(event.queryStringParameters?.limit) || 10;

    try {

        // List recent files from archive (get more than needed since we'll filter by user)
        const result = await s3.listObjectsV2({
            Bucket: bucketName,
            Prefix: 'archive/',
            MaxKeys: limit * 5 // Get more files since we'll filter by user
        }).promise();

        if (!result.Contents || result.Contents.length === 0) {
            return {
                statusCode: 200,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify([])
            };
        }

        // Sort by last modified (most recent first)
        const sortedFiles = result.Contents
            .filter(obj => obj.Key.endsWith('.json'))
            .sort((a, b) => {
                const dateA = new Date(b.LastModified);
                const dateB = new Date(a.LastModified);
                return dateA.getTime() - dateB.getTime();
            });

        // Get the actual change data and filter by user
        const changes = [];
        for (const file of sortedFiles) {
            // Stop if we have enough changes for this user
            if (changes.length >= limit) {
                break;
            }

            try {
                const getParams = {
                    Bucket: bucketName,
                    Key: file.Key
                };

                const data = await s3.getObject(getParams).promise();
                const change = JSON.parse(data.Body.toString());

                // ONLY include objects with changeId starting with CHG- AND belonging to current user
                const isUserChange = change.changeId && change.changeId.startsWith('CHG-') &&
                    (change.createdBy === userEmail ||
                        change.submittedBy === userEmail ||
                        change.modifiedBy === userEmail);

                if (isUserChange) {
                    change.lastModified = file.LastModified;
                    change.size = file.Size;
                    changes.push(change);
                }
            } catch (error) {
                console.error(`Error reading change file ${file.Key}:`, error);
            }
        }



        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify(changes)
        };

    } catch (error) {
        console.error('Error getting recent changes:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to get recent changes' })
        };
    }
}

// Authentication check endpoint
async function handleAuthCheck(event, userEmail) {
    return {
        statusCode: 200,
        headers: {
            'Content-Type': 'application/json',
            'Access-Control-Allow-Origin': '*'
        },
        body: JSON.stringify({
            authenticated: true,
            userEmail: userEmail,
            timestamp: toRFC3339(new Date())
        })
    };
}

// Get user context (role, permissions, customer affiliation)
async function handleGetUserContext(event, userEmail) {
    // Determine if user is admin based on email domain and role
    // For now, all @hearst.com users are considered admins
    const isAdmin = userEmail.toLowerCase().endsWith('@hearst.com');

    return {
        statusCode: 200,
        headers: {
            'Content-Type': 'application/json',
            'Access-Control-Allow-Origin': '*'
        },
        body: JSON.stringify({
            email: userEmail,
            isAdmin: isAdmin,
            role: isAdmin ? 'admin' : 'user',
            customerCode: null, // Could be populated from user attributes if needed
            timestamp: toRFC3339(new Date())
        })
    };
}

// Update existing change (for edit functionality)
async function handleUpdateChange(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').pop();
    const updatedChange = JSON.parse(event.body);

    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const archiveKey = `archive/${changeId}.json`;

    try {
        // First, get the existing change to verify ownership and get version info
        let existingChange;
        try {
            const getParams = {
                Bucket: bucketName,
                Key: archiveKey
            };
            const data = await s3.getObject(getParams).promise();
            existingChange = JSON.parse(data.Body.toString());
        } catch (error) {
            if (error.code === 'NoSuchKey') {
                return {
                    statusCode: 404,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Change not found' })
                };
            }
            throw error;
        }

        // Verify user can edit this change
        if (existingChange.createdBy !== userEmail && existingChange.submittedBy !== userEmail) {
            return {
                statusCode: 403,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Access denied to edit this change' })
            };
        }

        // Update change metadata
        const updateTimestamp = toRFC3339(new Date());
        updatedChange.changeId = changeId;
        updatedChange.createdBy = existingChange.createdBy;
        updatedChange.createdAt = existingChange.createdAt;
        updatedChange.submittedBy = existingChange.submittedBy;
        updatedChange.submittedAt = existingChange.submittedAt;
        updatedChange.version = (existingChange.version || 1) + 1;
        updatedChange.modifiedAt = updateTimestamp;
        updatedChange.modifiedBy = userEmail;
        updatedChange.status = updatedChange.status || existingChange.status || 'submitted';

        // Track prior status for status changes
        const oldStatus = existingChange.status;
        const newStatus = updatedChange.status;
        if (oldStatus !== newStatus) {
            updatedChange.prior_status = oldStatus;
        }

        // Add modification entry for status changes (matching announcement pattern)
        if (!updatedChange.modifications) {
            updatedChange.modifications = existingChange.modifications || [];
        }

        // If status changed to approved, add approval modification entry and set approval fields
        if (newStatus === 'approved' && oldStatus !== 'approved') {
            updatedChange.approvedAt = updateTimestamp;
            updatedChange.approvedBy = userEmail;
            updatedChange.modifications.push({
                timestamp: updateTimestamp,
                user_id: userEmail,
                modification_type: 'approved'
            });
        }
        // If status changed to cancelled, add cancellation modification entry
        else if (newStatus === 'cancelled' && oldStatus !== 'cancelled') {
            updatedChange.modifications.push({
                timestamp: updateTimestamp,
                user_id: userEmail,
                modification_type: 'cancelled'
            });
        }
        // If status changed to completed, add completion modification entry
        else if (newStatus === 'completed' && oldStatus !== 'completed') {
            updatedChange.modifications.push({
                timestamp: updateTimestamp,
                user_id: userEmail,
                modification_type: 'completed'
            });
        }



        // Save version history
        const versionKey = `versions/${changeId}/v${existingChange.version || 1}.json`;
        await s3.putObject({
            Bucket: bucketName,
            Key: versionKey,
            Body: JSON.stringify(existingChange, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': changeId,
                'version': String(existingChange.version || 1),
                'created-by': existingChange.createdBy
            }
        }).promise();

        // Update the main change record
        await s3.putObject({
            Bucket: bucketName,
            Key: archiveKey,
            Body: JSON.stringify(updatedChange, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': changeId,
                'version': String(updatedChange.version),
                'modified-by': userEmail,
                'status': updatedChange.status
            }
        }).promise();



        // Update customer buckets if customers changed
        if (updatedChange.customers && JSON.stringify(updatedChange.customers) !== JSON.stringify(existingChange.customers)) {
            await updateCustomerBuckets(updatedChange, existingChange);
        }

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                success: true,
                changeId: changeId,
                version: updatedChange.version,
                message: 'Change updated successfully'
            })
        };

    } catch (error) {
        console.error('Error updating change:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to update change' })
        };
    }
}

// Approve a change (change status to approved)
async function handleApproveChange(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').filter(p => p && p !== 'approve').pop();

    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const archiveKey = `archive/${changeId}.json`;

    try {
        // First, get the existing change
        let existingChange;
        try {
            const getParams = {
                Bucket: bucketName,
                Key: archiveKey
            };
            const data = await s3.getObject(getParams).promise();
            existingChange = JSON.parse(data.Body.toString());
        } catch (error) {
            if (error.code === 'NoSuchKey') {
                return {
                    statusCode: 404,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Change not found' })
                };
            }
            throw error;
        }

        // Check if change is in a state that can be approved
        if (existingChange.status === 'approved') {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Change is already approved' })
            };
        }

        if (existingChange.status === 'completed') {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Cannot approve a completed change' })
            };
        }

        // Update change with approval information
        const approvalTimestamp = toRFC3339(new Date());
        const approvedChange = {
            ...existingChange,
            prior_status: existingChange.status,
            status: 'approved',
            approvedAt: approvalTimestamp,
            approvedBy: userEmail,
            modifiedAt: approvalTimestamp,
            modifiedBy: userEmail,
            version: (existingChange.version || 1) + 1,
            // Ensure include_meeting field is preserved (default to false if not set)
            include_meeting: existingChange.include_meeting || false
        };

        // Add modification entry for approval (matching announcement pattern)
        if (!approvedChange.modifications) {
            approvedChange.modifications = [];
        }
        approvedChange.modifications.push({
            timestamp: approvalTimestamp,
            user_id: userEmail,
            modification_type: 'approved'
        });

        // Save version history before updating
        const versionKey = `versions/${changeId}/v${existingChange.version || 1}.json`;
        await s3.putObject({
            Bucket: bucketName,
            Key: versionKey,
            Body: JSON.stringify(existingChange, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': changeId,
                'version': String(existingChange.version || 1),
                'created-by': existingChange.createdBy || existingChange.submittedBy
            }
        }).promise();

        // Update the main change record with approval
        await s3.putObject({
            Bucket: bucketName,
            Key: archiveKey,
            Body: JSON.stringify(approvedChange, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': changeId,
                'version': String(approvedChange.version),
                'status': 'approved',
                'approved-by': userEmail,
                'approved-at': approvedChange.approvedAt
            }
        }).promise();

        // IMPORTANT: Upload approved change to customer prefixes to trigger S3 events for email notifications
        // The backend will send approval emails and schedule meetings based on request-type metadata
        if (approvedChange.customers && Array.isArray(approvedChange.customers)) {

            const customerUploadPromises = approvedChange.customers.map(async (customer) => {
                const customerKey = `customers/${customer}/${changeId}.json`;

                try {
                    await s3.putObject({
                        Bucket: bucketName,
                        Key: customerKey,
                        Body: JSON.stringify(approvedChange, null, 2),
                        ContentType: 'application/json',
                        Metadata: {
                            'change-id': changeId,
                            'customer-code': customer,
                            'status': 'approved',
                            'approved-by': userEmail,
                            'approved-at': approvedChange.approvedAt,
                            'request-type': 'approved_announcement'  // Backend uses this to route to approval handler
                        }
                    }).promise();

                    console.log(`✅ Created trigger for approved change: ${customerKey}`);
                    return { customer, success: true, key: customerKey };
                } catch (error) {
                    console.error(`❌ Failed to upload approved change to customer prefix ${customerKey}:`, error);
                    return { customer, success: false, error: error.message };
                }
            });

            const customerUploadResults = await Promise.allSettled(customerUploadPromises);
            const successfulUploads = customerUploadResults
                .filter(result => result.status === 'fulfilled' && result.value.success)
                .map(result => result.value);

            const failedUploads = customerUploadResults
                .filter(result => result.status === 'rejected' || (result.status === 'fulfilled' && !result.value.success));

            if (failedUploads.length > 0) {
                console.warn(`⚠️  ${failedUploads.length} customer prefix uploads failed - some customers may not receive approval notifications`);
            }
        } else {
            console.warn(`⚠️  No customers found in approved change - no email notifications will be sent`);
        }

        // After successful approval, clean up any corresponding draft to prevent duplicates
        try {
            const draftKey = `drafts/${changeId}.json`;

            // Check if draft exists
            try {
                await s3.headObject({
                    Bucket: bucketName,
                    Key: draftKey
                }).promise();

                // Draft exists, move it to deleted folder
                const draftData = await s3.getObject({
                    Bucket: bucketName,
                    Key: draftKey
                }).promise();

                const draft = JSON.parse(draftData.Body.toString());

                // Add approval metadata to the draft before moving to deleted
                draft.approvedAt = approvedChange.approvedAt;
                draft.approvedBy = userEmail;
                draft.deletedAt = toRFC3339(new Date());
                draft.deletedBy = userEmail;
                draft.deletionReason = 'approved';
                draft.originalPath = draftKey;

                // Move to deleted folder
                const deletedKey = `deleted/drafts/${changeId}.json`;
                await s3.putObject({
                    Bucket: bucketName,
                    Key: deletedKey,
                    Body: JSON.stringify(draft, null, 2),
                    ContentType: 'application/json',
                    Metadata: {
                        'change-id': draft.changeId,
                        'deleted-by': userEmail,
                        'deleted-at': draft.deletedAt,
                        'deletion-reason': 'approved',
                        'original-path': draftKey
                    }
                }).promise();

                // Delete the original draft
                await s3.deleteObject({
                    Bucket: bucketName,
                    Key: draftKey
                }).promise();



            } catch (error) {
                if (error.code === 'NotFound' || error.code === 'NoSuchKey') {
                    // No draft found - this is normal for changes that were directly submitted
                } else {
                    console.error(`⚠️ Failed to clean up draft ${changeId}:`, error);
                    // Don't fail the approval if draft cleanup fails
                }
            }
        } catch (error) {
            console.error('Error during draft cleanup after approval:', error);
            // Don't fail the approval if draft cleanup fails
        }

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                success: true,
                changeId: changeId,
                status: 'approved',
                approvedBy: userEmail,
                approvedAt: approvedChange.approvedAt,
                version: approvedChange.version,
                message: 'Change approved successfully'
            })
        };

    } catch (error) {
        console.error('Error approving change:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                error: 'Failed to approve change',
                message: error.message
            })
        };
    }
}

// Complete a change (change status to completed)
async function handleCompleteChange(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').filter(p => p && p !== 'complete').pop();

    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const archiveKey = `archive/${changeId}.json`;

    try {
        // First, get the existing change
        let existingChange;
        try {
            const getParams = {
                Bucket: bucketName,
                Key: archiveKey
            };
            const data = await s3.getObject(getParams).promise();
            existingChange = JSON.parse(data.Body.toString());
        } catch (error) {
            if (error.code === 'NoSuchKey') {
                return {
                    statusCode: 404,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Change not found' })
                };
            }
            throw error;
        }

        // Check if change is in a state that can be completed
        if (existingChange.status === 'completed') {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Change is already completed' })
            };
        }

        if (existingChange.status !== 'approved') {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Only approved changes can be completed' })
            };
        }

        // Update change with completion information
        const completedChange = {
            ...existingChange,
            prior_status: existingChange.status,
            status: 'completed',
            completedAt: toRFC3339(new Date()),
            completedBy: userEmail,
            modifiedAt: toRFC3339(new Date()),
            modifiedBy: userEmail,
            version: (existingChange.version || 1) + 1
        };

        // Save version history before updating
        const versionKey = `versions/${changeId}/v${existingChange.version || 1}.json`;
        await s3.putObject({
            Bucket: bucketName,
            Key: versionKey,
            Body: JSON.stringify(existingChange, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': changeId,
                'version': String(existingChange.version || 1),
                'created-by': existingChange.createdBy || existingChange.submittedBy
            }
        }).promise();

        // Update the main change record with completion
        await s3.putObject({
            Bucket: bucketName,
            Key: archiveKey,
            Body: JSON.stringify(completedChange, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': changeId,
                'version': String(completedChange.version),
                'status': 'completed',
                'completed-by': userEmail,
                'completed-at': completedChange.completedAt
            }
        }).promise();

        // IMPORTANT: Upload completed change to customer prefixes to trigger S3 events for completion email notifications
        if (completedChange.customers && Array.isArray(completedChange.customers)) {

            const customerUploadPromises = completedChange.customers.map(async (customer) => {
                const customerKey = `customers/${customer}/${changeId}.json`;

                try {
                    await s3.putObject({
                        Bucket: bucketName,
                        Key: customerKey,
                        Body: JSON.stringify(completedChange, null, 2),
                        ContentType: 'application/json',
                        Metadata: {
                            'change-id': changeId,
                            'customer-code': customer,
                            'status': 'completed',
                            'completed-by': userEmail,
                            'completed-at': completedChange.completedAt,
                            'request-type': 'change_complete'  // This tells the backend what type of email to send
                        }
                    }).promise();


                    return { customer, success: true, key: customerKey };
                } catch (error) {
                    console.error(`❌ Failed to upload completed change to customer prefix ${customerKey}:`, error);
                    return { customer, success: false, error: error.message };
                }
            });

            const customerUploadResults = await Promise.allSettled(customerUploadPromises);
            const successfulUploads = customerUploadResults
                .filter(result => result.status === 'fulfilled' && result.value.success)
                .map(result => result.value);

            const failedUploads = customerUploadResults
                .filter(result => result.status === 'rejected' || (result.status === 'fulfilled' && !result.value.success));



            if (failedUploads.length > 0) {
                console.warn(`⚠️  ${failedUploads.length} customer prefix uploads failed - some customers may not receive completion notifications`);
            }
        } else {
            console.warn(`⚠️  No customers found in completed change - no completion email notifications will be sent`);
        }

        // After successful completion, clean up any corresponding draft to prevent duplicates
        try {
            const draftKey = `drafts/${changeId}.json`;

            // Check if draft exists
            try {
                await s3.headObject({
                    Bucket: bucketName,
                    Key: draftKey
                }).promise();

                // Draft exists, move it to deleted folder
                const draftData = await s3.getObject({
                    Bucket: bucketName,
                    Key: draftKey
                }).promise();

                const draft = JSON.parse(draftData.Body.toString());

                // Add completion metadata to the draft before moving to deleted
                draft.completedAt = completedChange.completedAt;
                draft.completedBy = userEmail;
                draft.deletedAt = toRFC3339(new Date());
                draft.deletedBy = userEmail;
                draft.deletionReason = 'completed';
                draft.originalPath = draftKey;

                // Move to deleted folder
                const deletedKey = `deleted/drafts/${changeId}.json`;
                await s3.putObject({
                    Bucket: bucketName,
                    Key: deletedKey,
                    Body: JSON.stringify(draft, null, 2),
                    ContentType: 'application/json',
                    Metadata: {
                        'change-id': draft.changeId,
                        'deleted-by': userEmail,
                        'deleted-at': draft.deletedAt,
                        'deletion-reason': 'completed',
                        'original-path': draftKey
                    }
                }).promise();

                // Delete the original draft
                await s3.deleteObject({
                    Bucket: bucketName,
                    Key: draftKey
                }).promise();



            } catch (error) {
                if (error.code === 'NotFound' || error.code === 'NoSuchKey') {
                    // No draft found - this is normal for changes that were directly submitted
                } else {
                    console.error(`⚠️ Failed to clean up draft ${changeId}:`, error);
                    // Don't fail the completion if draft cleanup fails
                }
            }
        } catch (error) {
            console.error('Error during draft cleanup after completion:', error);
            // Don't fail the completion if draft cleanup fails
        }

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                success: true,
                changeId: changeId,
                status: 'completed',
                completedBy: userEmail,
                completedAt: completedChange.completedAt,
                version: completedChange.version,
                message: 'Change completed successfully'
            })
        };

    } catch (error) {
        console.error('Error completing change:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                error: 'Failed to complete change',
                message: error.message
            })
        };
    }
}

// Cancel a change (change status to cancelled and cancel any meetings)
async function handleCancelChange(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').filter(p => p && p !== 'cancel').pop();

    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const archiveKey = `archive/${changeId}.json`;

    try {
        // First, get the existing change
        let existingChange;
        try {
            const getParams = {
                Bucket: bucketName,
                Key: archiveKey
            };
            const data = await s3.getObject(getParams).promise();
            existingChange = JSON.parse(data.Body.toString());
            console.log('📋 Loaded change from S3 for cancellation');
            console.log('📋 Change has meeting_id:', !!existingChange.meeting_id);
            console.log('📋 Change has join_url:', !!existingChange.join_url);
        } catch (error) {
            if (error.code === 'NoSuchKey') {
                return {
                    statusCode: 404,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Change not found' })
                };
            }
            throw error;
        }

        // Check if change is in a state that can be cancelled
        if (existingChange.status === 'cancelled') {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Change is already cancelled' })
            };
        }

        if (existingChange.status === 'completed') {
            return {
                statusCode: 400,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify({ error: 'Cannot cancel a completed change' })
            };
        }

        // Update change with cancellation information
        const cancelledChange = {
            ...existingChange,
            prior_status: existingChange.status,
            status: 'cancelled',
            cancelledAt: toRFC3339(new Date()),
            cancelledBy: userEmail,
            modifiedAt: toRFC3339(new Date()),
            modifiedBy: userEmail,
            version: (existingChange.version || 1) + 1
        };

        // Save version history before updating
        const versionKey = `versions/${changeId}/v${existingChange.version || 1}.json`;
        await s3.putObject({
            Bucket: bucketName,
            Key: versionKey,
            Body: JSON.stringify(existingChange, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': changeId,
                'version': String(existingChange.version || 1),
                'created-by': existingChange.createdBy || existingChange.submittedBy
            }
        }).promise();

        // Update the main change record with cancellation
        await s3.putObject({
            Bucket: bucketName,
            Key: archiveKey,
            Body: JSON.stringify(cancelledChange, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': changeId,
                'version': String(cancelledChange.version),
                'status': 'cancelled',
                'cancelled-by': userEmail,
                'cancelled-at': cancelledChange.cancelledAt
            }
        }).promise();

        // Upload cancelled change to customer prefixes to trigger S3 events for cancellation notifications
        if (cancelledChange.customers && Array.isArray(cancelledChange.customers)) {
            const customerUploadPromises = cancelledChange.customers.map(async (customer) => {
                const customerKey = `customers/${customer}/${changeId}.json`;

                try {
                    await s3.putObject({
                        Bucket: bucketName,
                        Key: customerKey,
                        Body: JSON.stringify(cancelledChange, null, 2),
                        ContentType: 'application/json',
                        Metadata: {
                            'change-id': changeId,
                            'customer-code': customer,
                            'status': 'cancelled',
                            'cancelled-by': userEmail,
                            'cancelled-at': cancelledChange.cancelledAt,
                            'request-type': 'change_cancelled'  // This tells the backend to cancel meetings
                        }
                    }).promise();

                    return { customer, success: true, key: customerKey };
                } catch (error) {
                    console.error(`❌ Failed to upload cancelled change to customer prefix ${customerKey}:`, error);
                    return { customer, success: false, error: error.message };
                }
            });

            const customerUploadResults = await Promise.allSettled(customerUploadPromises);
            const successfulUploads = customerUploadResults
                .filter(result => result.status === 'fulfilled' && result.value.success)
                .map(result => result.value);

            const failedUploads = customerUploadResults
                .filter(result => result.status === 'rejected' || (result.status === 'fulfilled' && !result.value.success));

            if (failedUploads.length > 0) {
                console.warn(`⚠️  ${failedUploads.length} customer prefix uploads failed - some customers may not receive cancellation notifications`);
            }
        } else {
            console.warn(`⚠️  No customers found in cancelled change - no cancellation notifications will be sent`);
        }

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                success: true,
                changeId: changeId,
                status: 'cancelled',
                cancelledBy: userEmail,
                cancelledAt: cancelledChange.cancelledAt,
                version: cancelledChange.version,
                message: 'Change cancelled successfully'
            })
        };

    } catch (error) {
        console.error('Error cancelling change:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                error: 'Failed to cancel change',
                message: error.message
            })
        };
    }
}

// Get version history for a change
async function handleGetChangeVersions(event, userEmail) {
    const pathParts = (event.path || event.rawPath).split('/');
    const changeId = pathParts[pathParts.indexOf('changes') + 1] || pathParts[pathParts.indexOf('api') + 2];

    // Check if requesting specific version
    const versionNumber = pathParts[pathParts.length - 1];
    const isSpecificVersion = !isNaN(versionNumber) && pathParts.includes('versions');

    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';

    try {
        if (isSpecificVersion) {
            // Get specific version
            const versionKey = `versions/${changeId}/v${versionNumber}.json`;

            try {
                const data = await s3.getObject({
                    Bucket: bucketName,
                    Key: versionKey
                }).promise();

                const version = JSON.parse(data.Body.toString());

                return {
                    statusCode: 200,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify(version)
                };
            } catch (error) {
                if (error.code === 'NoSuchKey') {
                    return {
                        statusCode: 404,
                        headers: {
                            'Content-Type': 'application/json',
                            'Access-Control-Allow-Origin': '*'
                        },
                        body: JSON.stringify({ error: 'Version not found' })
                    };
                }
                throw error;
            }
        } else {
            // Get version history list
            const prefix = `versions/${changeId}/`;

            const result = await s3.listObjectsV2({
                Bucket: bucketName,
                Prefix: prefix
            }).promise();

            const versions = [];

            for (const object of result.Contents || []) {
                try {
                    const data = await s3.getObject({
                        Bucket: bucketName,
                        Key: object.Key
                    }).promise();

                    const version = JSON.parse(data.Body.toString());
                    version.lastModified = object.LastModified;
                    version.size = object.Size;

                    versions.push(version);
                } catch (error) {
                    console.error(`Error reading version file ${object.Key}:`, error);
                }
            }

            // Sort by version number (newest first)
            versions.sort((a, b) => (b.version || 0) - (a.version || 0));

            return {
                statusCode: 200,
                headers: {
                    'Content-Type': 'application/json',
                    'Access-Control-Allow-Origin': '*'
                },
                body: JSON.stringify(versions)
            };
        }

    } catch (error) {
        console.error('Error getting change versions:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to get change versions' })
        };
    }
}

// Delete draft
async function handleDeleteDraft(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').pop();
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `drafts/${changeId}.json`;

    try {
        // First verify the draft exists and user owns it
        try {
            const data = await s3.getObject({
                Bucket: bucketName,
                Key: key
            }).promise();

            const draft = JSON.parse(data.Body.toString());

            if (draft.createdBy !== userEmail && draft.submittedBy !== userEmail) {
                return {
                    statusCode: 403,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Access denied to delete this draft' })
                };
            }
        } catch (error) {
            if (error.code === 'NoSuchKey') {
                return {
                    statusCode: 404,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Draft not found' })
                };
            }
            throw error;
        }

        // Move the draft to deleted folder instead of permanently deleting
        const deletedKey = `deleted/drafts/${changeId}.json`;

        // First get the draft content
        const draftData = await s3.getObject({
            Bucket: bucketName,
            Key: key
        }).promise();

        const draft = JSON.parse(draftData.Body.toString());

        // Add deletion metadata
        draft.deletedAt = toRFC3339(new Date());
        draft.deletedBy = userEmail;
        draft.originalPath = key;

        // Copy to deleted folder
        await s3.putObject({
            Bucket: bucketName,
            Key: deletedKey,
            Body: JSON.stringify(draft, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': draft.changeId,
                'deleted-by': userEmail,
                'deleted-at': draft.deletedAt,
                'original-path': key
            }
        }).promise();

        // Now delete the original
        await s3.deleteObject({
            Bucket: bucketName,
            Key: key
        }).promise();

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                success: true,
                message: 'Draft moved to deleted folder successfully',
                deletedPath: deletedKey
            })
        };

    } catch (error) {
        console.error('Error deleting draft:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to delete draft' })
        };
    }
}

// Delete submitted change
async function handleDeleteChange(event, userEmail) {
    const changeId = event.pathParameters?.changeId || (event.path || event.rawPath).split('/').pop();
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `archive/${changeId}.json`;

    try {
        // Parse request body to get the full change object with top-level meeting fields
        let changeFromRequest = null;
        if (event.body) {
            try {
                changeFromRequest = JSON.parse(event.body);
                console.log('Received change from request body');
                console.log('Has meeting_id:', !!changeFromRequest.meeting_id);
                console.log('Has join_url:', !!changeFromRequest.join_url);
            } catch (parseError) {
                console.warn('Failed to parse request body:', parseError);
            }
        }

        // First verify the change exists and user owns it
        let change;
        try {
            const data = await s3.getObject({
                Bucket: bucketName,
                Key: key
            }).promise();

            change = JSON.parse(data.Body.toString());
            console.log('📋 Loaded change from S3 for deletion');
            console.log('📋 Change has meeting_id:', !!change.meeting_id);
            console.log('📋 Change has join_url:', !!change.join_url);

            if (change.createdBy !== userEmail && change.submittedBy !== userEmail) {
                return {
                    statusCode: 403,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Access denied to delete this change' })
                };
            }
        } catch (error) {
            if (error.code === 'NoSuchKey') {
                return {
                    statusCode: 404,
                    headers: {
                        'Content-Type': 'application/json',
                        'Access-Control-Allow-Origin': '*'
                    },
                    body: JSON.stringify({ error: 'Change not found' })
                };
            }
            throw error;
        }

        // Merge top-level meeting fields from request body (latest from S3)
        if (changeFromRequest) {
            if (changeFromRequest.meeting_id) {
                change.meeting_id = changeFromRequest.meeting_id;
                console.log('Preserved meeting_id:', change.meeting_id);
            }
            if (changeFromRequest.join_url) {
                change.join_url = changeFromRequest.join_url;
                console.log('Preserved join_url');
            }
        }

        // Move the change to deleted folder instead of permanently deleting
        const deletedKey = `deleted/archive/${changeId}.json`;

        // Add deletion metadata
        change.deletedAt = toRFC3339(new Date());
        change.deletedBy = userEmail;
        change.originalPath = key;

        // Copy to deleted folder
        await s3.putObject({
            Bucket: bucketName,
            Key: deletedKey,
            Body: JSON.stringify(change, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': change.changeId,
                'deleted-by': userEmail,
                'deleted-at': change.deletedAt,
                'original-path': key
            }
        }).promise();

        // Also move from customer buckets if they exist
        if (change.customers && Array.isArray(change.customers)) {
            for (const customer of change.customers) {
                const customerKey = `customers/${customer}/${changeId}.json`;
                const deletedCustomerKey = `deleted/customers/${customer}/${changeId}.json`;

                try {
                    // Check if customer file exists
                    const customerData = await s3.getObject({
                        Bucket: bucketName,
                        Key: customerKey
                    }).promise();

                    // Copy to deleted folder
                    await s3.putObject({
                        Bucket: bucketName,
                        Key: deletedCustomerKey,
                        Body: customerData.Body,
                        ContentType: 'application/json',
                        Metadata: {
                            'change-id': changeId,
                            'customer': customer,
                            'deleted-by': userEmail,
                            'deleted-at': change.deletedAt,
                            'original-path': customerKey
                        }
                    }).promise();

                    // Delete original customer file
                    await s3.deleteObject({
                        Bucket: bucketName,
                        Key: customerKey
                    }).promise();


                } catch (customerError) {
                    if (customerError.code !== 'NoSuchKey') {
                        console.error(`Error moving customer file ${customerKey}:`, customerError);
                    }
                }
            }
        }

        // Now delete the original archive file
        await s3.deleteObject({
            Bucket: bucketName,
            Key: key
        }).promise();

        return {
            statusCode: 200,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({
                success: true,
                message: 'Change moved to deleted folder successfully',
                deletedPath: deletedKey
            })
        };

    } catch (error) {
        console.error('Error deleting change:', error);
        return {
            statusCode: 500,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Failed to delete change' })
        };
    }
}

// Helper function to update customer buckets when customers change
async function updateCustomerBuckets(updatedChange, existingChange) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';

    // Note: Editing a change should NOT create trigger files in customers/ path
    // Trigger files are only created for workflow transitions (submit, approve, complete, cancel)
    // The archive update is sufficient for edit operations
    console.log(`✅ Change updated in archive - no triggers created for edit operation`);
}