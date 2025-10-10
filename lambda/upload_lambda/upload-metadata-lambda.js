import AWS from 'aws-sdk';
import { DateTime, parseDateTime, toRFC3339, toLogFormat, validateDateTime, validateMeetingTime, ERROR_TYPES } from './datetime/index.js';

// Initialize AWS services
const s3 = new AWS.S3();
const sqs = new AWS.SQS();

// Initialize datetime utilities with default config
const dateTime = new DateTime();

export const handler = async (event) => {
    try {
        // Validate authentication headers added by Lambda@Edge SAML function
        const userEmail = event.headers['x-user-email'];
        const isAuthenticated = event.headers['x-authenticated'] === 'true';

        if (!isAuthenticated || !userEmail) {
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
        } else if ((path === '/changes/statistics' && method === 'GET') || (path === '/api/changes/statistics' && method === 'GET')) {
            return await handleGetStatistics(event, userEmail);
        } else if ((path === '/changes/recent' && method === 'GET') || (path === '/api/changes/recent' && method === 'GET')) {
            return await handleGetRecentChanges(event, userEmail);
        } else if (path.startsWith('/api/changes/') && path.includes('/versions') && method === 'GET') {
            return await handleGetChangeVersions(event, userEmail);
        } else if (path.startsWith('/api/changes/') && method === 'GET') {
            return await handleGetChange(event, userEmail);
        } else if (path.startsWith('/api/changes/') && method === 'PUT') {
            return await handleUpdateChange(event, userEmail);
        } else if (path.startsWith('/api/drafts/') && method === 'GET') {
            return await handleGetDraft(event, userEmail);
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

// Original upload handler
async function handleUpload(event, userEmail) {
    const metadata = JSON.parse(event.body);

    // Validate required fields
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

    // Validate date/time fields if present
    try {
        let startDate = null;
        let endDate = null;
        
        if (metadata.implementationStart) {
            startDate = parseDateTime(metadata.implementationStart);
            validateDateTime(startDate);
            metadata.implementationStart = toRFC3339(startDate);
        }
        
        if (metadata.implementationEnd) {
            endDate = parseDateTime(metadata.implementationEnd);
            validateDateTime(endDate);
            metadata.implementationEnd = toRFC3339(endDate);
        }
        
        // Validate date range if both dates are provided
        if (startDate && endDate) {
            dateTime.validateDateRange(startDate, endDate);
        }
        
        // Validate meeting times if present
        if (metadata.meetingTime) {
            const meetingDate = parseDateTime(metadata.meetingTime);
            validateMeetingTime(meetingDate);
            metadata.meetingTime = toRFC3339(meetingDate);
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

    // Only generate new ID if one doesn't exist (preserve draft IDs)
    if (!metadata.changeId) {
        metadata.changeId = generateChangeId();
    }

    metadata.status = 'submitted';

    // Set metadata for processing
    if (!metadata.metadata) {
        metadata.metadata = {};
    }
    metadata.metadata.status = 'submitted';
    metadata.metadata.request_type = 'approval_request';

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

    return {
        statusCode: 200,
        headers: {
            'Content-Type': 'application/json',
            'Access-Control-Allow-Origin': '*'
        },
        body: JSON.stringify({
            success: true,
            changeId: metadata.changeId,
            uploadResults: uploadResults,
            summary: {
                total: uploadResults.length,
                successful: successCount,
                failed: failureCount
            }
        })
    };
}

// Get all changes (for view-changes page)
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

        // Get metadata for each change file
        for (const object of result.Contents) {
            try {
                const getParams = {
                    Bucket: bucketName,
                    Key: object.Key
                };

                const data = await s3.getObject(getParams).promise();
                const change = JSON.parse(data.Body.toString());

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

                // Only include changes created by this user
                if (change.createdBy === userEmail || change.submittedBy === userEmail) {
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

    // Validate required fields
    if (!draft.changeId) {
        return {
            statusCode: 400,
            headers: {
                'Content-Type': 'application/json',
                'Access-Control-Allow-Origin': '*'
            },
            body: JSON.stringify({ error: 'Missing required field: changeId' })
        };
    }

    // Validate and normalize date/time fields if present
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

    // Add/update user context
    draft.status = 'draft';
    draft.modifiedAt = toRFC3339(new Date());
    draft.modifiedBy = userEmail;

    if (!draft.createdAt) {
        draft.createdAt = draft.modifiedAt;
        draft.createdBy = userEmail;
    }

    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `drafts/${draft.changeId}.json`;

    try {
        const params = {
            Bucket: bucketName,
            Key: key,
            Body: JSON.stringify(draft, null, 2),
            ContentType: 'application/json',
            Metadata: {
                'change-id': draft.changeId,
                'created-by': draft.createdBy,
                'modified-by': draft.modifiedBy,
                'status': 'draft'
            }
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

async function uploadToCustomerBuckets(metadata) {
    const uploadPromises = [];

    for (const customer of metadata.customers) {
        uploadPromises.push(uploadToCustomerBucket(metadata, customer));
    }

    uploadPromises.push(uploadToArchiveBucket(metadata));

    const results = await Promise.allSettled(uploadPromises);

    return results.map((result, index) => {
        const isArchive = index === metadata.customers.length;
        const customerName = isArchive ? 'Archive (Permanent Storage)' : getCustomerDisplayName(metadata.customers[index]);

        if (result.status === 'fulfilled') {
            return {
                customer: customerName,
                success: true,
                s3Key: result.value.key,
                bucket: result.value.bucket
            };
        } else {
            return {
                customer: customerName,
                success: false,
                error: result.reason.message
            };
        }
    });
}

async function uploadToCustomerBucket(metadata, customer) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `customers/${customer}/${metadata.changeId}.json`;

    const params = {
        Bucket: bucketName,
        Key: key,
        Body: JSON.stringify(metadata, null, 2),
        ContentType: 'application/json',
        Metadata: {
            'change-id': metadata.changeId,
            'customer': customer,
            'submitted-by': metadata.submittedBy,
            'submitted-at': metadata.submittedAt
        }
    };

    await s3.putObject(params).promise();
    return { bucket: bucketName, key: key };
}

async function uploadToArchiveBucket(metadata) {
    const bucketName = process.env.S3_BUCKET_NAME || '4cm-prod-ccoe-change-management-metadata';
    const key = `archive/${metadata.changeId}.json`;

    const params = {
        Bucket: bucketName,
        Key: key,
        Body: JSON.stringify(metadata, null, 2),
        ContentType: 'application/json',
        Metadata: {
            'change-id': metadata.changeId,
            'submitted-by': metadata.submittedBy,
            'submitted-at': metadata.submittedAt,
            'customers': metadata.customers.join(',')
        }
    };

    await s3.putObject(params).promise();
    return { bucket: bucketName, key: key };
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
                    
                    // Check if this change belongs to the current user
                    if (change.createdBy === userEmail || change.submittedBy === userEmail || change.modifiedBy === userEmail) {
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
                    
                    // Check if this change belongs to the current user
                    const isUserChange = change.createdBy === userEmail || 
                                       change.submittedBy === userEmail || 
                                       change.modifiedBy === userEmail;
                    
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

                // Check if this change belongs to the current user
                const isUserChange = change.createdBy === userEmail || 
                                   change.submittedBy === userEmail || 
                                   change.modifiedBy === userEmail;

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
        updatedChange.changeId = changeId;
        updatedChange.createdBy = existingChange.createdBy;
        updatedChange.createdAt = existingChange.createdAt;
        updatedChange.submittedBy = existingChange.submittedBy;
        updatedChange.submittedAt = existingChange.submittedAt;
        updatedChange.version = (existingChange.version || 1) + 1;
        updatedChange.modifiedAt = toRFC3339(new Date());
        updatedChange.modifiedBy = userEmail;
        updatedChange.status = updatedChange.status || existingChange.status || 'submitted';
        


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
        const approvedChange = {
            ...existingChange,
            status: 'approved',
            approvedAt: toRFC3339(new Date()),
            approvedBy: userEmail,
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
                            'request-type': 'approved_announcement'  // This tells the backend what type of email to send
                        }
                    }).promise();
                    

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
        // First verify the change exists and user owns it
        try {
            const data = await s3.getObject({
                Bucket: bucketName,
                Key: key
            }).promise();

            const change = JSON.parse(data.Body.toString());

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

        // Move the change to deleted folder instead of permanently deleting
        const deletedKey = `deleted/archive/${changeId}.json`;
        
        // First get the change content
        const changeData = await s3.getObject({
            Bucket: bucketName,
            Key: key
        }).promise();
        
        const change = JSON.parse(changeData.Body.toString());
        
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

    // Remove from old customer buckets
    const oldCustomers = existingChange.customers || [];
    const newCustomers = updatedChange.customers || [];

    // Delete from customers no longer in the list
    for (const customer of oldCustomers) {
        if (!newCustomers.includes(customer)) {
            try {
                await s3.deleteObject({
                    Bucket: bucketName,
                    Key: `customers/${customer}/${updatedChange.changeId}.json`
                }).promise();

            } catch (error) {
                console.error(`Error removing from customer bucket ${customer}:`, error);
            }
        }
    }

    // Add to new customer buckets
    for (const customer of newCustomers) {
        try {
            await s3.putObject({
                Bucket: bucketName,
                Key: `customers/${customer}/${updatedChange.changeId}.json`,
                Body: JSON.stringify(updatedChange, null, 2),
                ContentType: 'application/json',
                Metadata: {
                    'change-id': updatedChange.changeId,
                    'customer': customer,
                    'modified-by': updatedChange.modifiedBy,
                    'modified-at': updatedChange.modifiedAt
                }
            }).promise();

        } catch (error) {
            console.error(`Error adding to customer bucket ${customer}:`, error);
        }
    }
}