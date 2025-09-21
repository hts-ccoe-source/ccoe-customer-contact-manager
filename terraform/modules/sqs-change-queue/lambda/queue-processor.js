// Lambda function for processing SQS messages in customer accounts
// Handles change notifications for multi-customer email distribution

const AWS = require('aws-sdk');

// Initialize AWS services
const ses = new AWS.SES();
const s3 = new AWS.S3();

// Configuration from environment variables
const CUSTOMER_CODE = process.env.CUSTOMER_CODE;
const LOG_LEVEL = process.env.LOG_LEVEL || 'info';

exports.handler = async (event) => {
    console.log(`Processing ${event.Records.length} messages for customer: ${CUSTOMER_CODE}`);
    
    const results = {
        successful: 0,
        failed: 0,
        errors: []
    };
    
    for (const record of event.Records) {
        try {
            await processMessage(record);
            results.successful++;
        } catch (error) {
            console.error('Error processing message:', error);
            results.failed++;
            results.errors.push({
                messageId: record.messageId,
                error: error.message
            });
        }
    }
    
    console.log('Processing results:', results);
    
    // If any messages failed, throw an error to trigger retry
    if (results.failed > 0) {
        throw new Error(`Failed to process ${results.failed} out of ${event.Records.length} messages`);
    }
    
    return results;
};

async function processMessage(record) {
    const messageBody = JSON.parse(record.body);
    
    // Handle S3 event notifications
    if (messageBody.Records && messageBody.Records[0].eventSource === 'aws:s3') {
        return await processS3Event(messageBody.Records[0]);
    }
    
    // Handle direct change notifications
    if (messageBody.change_id) {
        return await processChangeNotification(messageBody);
    }
    
    throw new Error('Unknown message format');
}

async function processS3Event(s3Record) {
    const bucket = s3Record.s3.bucket.name;
    const key = decodeURIComponent(s3Record.s3.object.key.replace(/\+/g, ' '));
    
    console.log(`Processing S3 event: ${bucket}/${key}`);
    
    // Download metadata file from S3
    const s3Object = await s3.getObject({
        Bucket: bucket,
        Key: key
    }).promise();
    
    const metadata = JSON.parse(s3Object.Body.toString());
    
    // Validate that this customer should process this change
    if (!metadata.customer_codes || !metadata.customer_codes.includes(CUSTOMER_CODE)) {
        console.log(`Skipping change ${metadata.change_id} - not for customer ${CUSTOMER_CODE}`);
        return;
    }
    
    return await processChangeNotification(metadata);
}

async function processChangeNotification(metadata) {
    console.log(`Processing change notification: ${metadata.change_id}`);
    
    // Validate required fields
    if (!metadata.change_id || !metadata.title || !metadata.template_id) {
        throw new Error('Missing required fields in change notification');
    }
    
    // Get email template
    const template = await getEmailTemplate(metadata.template_id);
    
    // Get customer email lists
    const emailLists = await getCustomerEmailLists();
    
    // Process emails for each list
    const emailResults = [];
    for (const emailList of emailLists) {
        try {
            const result = await sendEmailsToList(emailList, template, metadata);
            emailResults.push(result);
        } catch (error) {
            console.error(`Failed to send emails to list ${emailList.name}:`, error);
            emailResults.push({
                listName: emailList.name,
                success: false,
                error: error.message
            });
        }
    }
    
    // Log results
    const totalSent = emailResults.reduce((sum, result) => sum + (result.sent || 0), 0);
    const totalFailed = emailResults.reduce((sum, result) => sum + (result.failed || 0), 0);
    
    console.log(`Change ${metadata.change_id} processed: ${totalSent} sent, ${totalFailed} failed`);
    
    return {
        changeId: metadata.change_id,
        customerCode: CUSTOMER_CODE,
        emailsSent: totalSent,
        emailsFailed: totalFailed,
        results: emailResults
    };
}

async function getEmailTemplate(templateId) {
    // In a real implementation, this would fetch from a template store
    // For now, return a basic template structure
    return {
        id: templateId,
        subject: 'Change Notification: {{title}}',
        htmlBody: `
            <html>
            <body>
                <h2>{{title}}</h2>
                <p>{{description}}</p>
                <p>Change ID: {{change_id}}</p>
                <p>Priority: {{priority}}</p>
                {{#if email_data.message}}
                <div>
                    <h3>Additional Information:</h3>
                    <p>{{email_data.message}}</p>
                </div>
                {{/if}}
            </body>
            </html>
        `,
        textBody: `
            {{title}}
            
            {{description}}
            
            Change ID: {{change_id}}
            Priority: {{priority}}
            
            {{#if email_data.message}}
            Additional Information:
            {{email_data.message}}
            {{/if}}
        `
    };
}

async function getCustomerEmailLists() {
    // In a real implementation, this would fetch from a database or configuration
    // For now, return mock data
    return [
        {
            name: 'primary-notifications',
            emails: [
                'admin@customer.com',
                'notifications@customer.com'
            ]
        },
        {
            name: 'emergency-contacts',
            emails: [
                'emergency@customer.com',
                'oncall@customer.com'
            ]
        }
    ];
}

async function sendEmailsToList(emailList, template, metadata) {
    const results = {
        listName: emailList.name,
        sent: 0,
        failed: 0,
        errors: []
    };
    
    // Render template with metadata
    const renderedSubject = renderTemplate(template.subject, metadata);
    const renderedHtmlBody = renderTemplate(template.htmlBody, metadata);
    const renderedTextBody = renderTemplate(template.textBody, metadata);
    
    // Send emails to each recipient
    for (const email of emailList.emails) {
        try {
            await ses.sendEmail({
                Source: `noreply@${CUSTOMER_CODE}.example.com`,
                Destination: {
                    ToAddresses: [email]
                },
                Message: {
                    Subject: {
                        Data: renderedSubject,
                        Charset: 'UTF-8'
                    },
                    Body: {
                        Html: {
                            Data: renderedHtmlBody,
                            Charset: 'UTF-8'
                        },
                        Text: {
                            Data: renderedTextBody,
                            Charset: 'UTF-8'
                        }
                    }
                }
            }).promise();
            
            results.sent++;
            console.log(`Email sent to ${email} for change ${metadata.change_id}`);
            
        } catch (error) {
            results.failed++;
            results.errors.push({
                email: email,
                error: error.message
            });
            console.error(`Failed to send email to ${email}:`, error);
        }
    }
    
    return results;
}

function renderTemplate(template, data) {
    // Simple template rendering - in production, use a proper template engine
    let rendered = template;
    
    // Replace simple variables
    rendered = rendered.replace(/\{\{(\w+)\}\}/g, (match, key) => {
        return data[key] || match;
    });
    
    // Replace nested variables
    rendered = rendered.replace(/\{\{(\w+)\.(\w+)\}\}/g, (match, obj, key) => {
        return (data[obj] && data[obj][key]) || match;
    });
    
    // Handle simple conditionals
    rendered = rendered.replace(/\{\{#if (\w+)\.(\w+)\}\}(.*?)\{\{\/if\}\}/gs, (match, obj, key, content) => {
        return (data[obj] && data[obj][key]) ? content : '';
    });
    
    return rendered;
}