// Lambda@Edge function for Identity Center authentication
// Handles authentication flow for multi-customer email distribution portal

const crypto = require('crypto');
const querystring = require('querystring');

// Configuration from environment variables
const IDENTITY_CENTER_DOMAIN = process.env.IDENTITY_CENTER_DOMAIN;
const ALLOWED_GROUPS = JSON.parse(process.env.ALLOWED_GROUPS || '[]');
const SESSION_DURATION = parseInt(process.env.SESSION_DURATION || '3600');
const COOKIE_DOMAIN = process.env.COOKIE_DOMAIN;
const REDIRECT_URI = process.env.REDIRECT_URI;

// Cookie names
const SESSION_COOKIE = 'mc-email-session';
const STATE_COOKIE = 'mc-email-state';

exports.handler = async (event) => {
    const request = event.Records[0].cf.request;
    const headers = request.headers;
    
    try {
        // Check if this is a callback from Identity Center
        if (request.uri === '/auth/callback') {
            return await handleAuthCallback(request);
        }
        
        // Check if this is a logout request
        if (request.uri === '/auth/logout') {
            return handleLogout();
        }
        
        // Check for existing valid session
        const sessionCookie = getCookie(headers, SESSION_COOKIE);
        if (sessionCookie && await isValidSession(sessionCookie)) {
            // Valid session, allow request to continue
            return request;
        }
        
        // No valid session, redirect to Identity Center
        return redirectToIdentityCenter(request);
        
    } catch (error) {
        console.error('Authentication error:', error);
        return createErrorResponse(500, 'Authentication Error');
    }
};

async function handleAuthCallback(request) {
    const queryParams = querystring.parse(request.querystring);
    const code = queryParams.code;
    const state = queryParams.state;
    
    if (!code || !state) {
        return createErrorResponse(400, 'Missing authorization code or state');
    }
    
    // Verify state parameter
    const stateCookie = getCookie(request.headers, STATE_COOKIE);
    if (!stateCookie || stateCookie !== state) {
        return createErrorResponse(400, 'Invalid state parameter');
    }
    
    try {
        // Exchange code for tokens
        const tokens = await exchangeCodeForTokens(code);
        
        // Validate user and get groups
        const userInfo = await validateUserAndGetGroups(tokens.access_token);
        
        // Check if user is in allowed groups
        if (!isUserAuthorized(userInfo.groups)) {
            return createErrorResponse(403, 'Access Denied - Insufficient Permissions');
        }
        
        // Create session
        const sessionData = {
            userId: userInfo.userId,
            email: userInfo.email,
            groups: userInfo.groups,
            customerCodes: extractCustomerCodes(userInfo.groups),
            expiresAt: Date.now() + (SESSION_DURATION * 1000)
        };
        
        const sessionToken = createSessionToken(sessionData);
        
        // Redirect to original URL or home
        const originalUrl = getCookie(request.headers, 'mc-email-original-url') || '/';
        
        return {
            status: '302',
            statusDescription: 'Found',
            headers: {
                location: [{
                    key: 'Location',
                    value: originalUrl
                }],
                'set-cookie': [
                    {
                        key: 'Set-Cookie',
                        value: `${SESSION_COOKIE}=${sessionToken}; Domain=${COOKIE_DOMAIN}; Path=/; Secure; HttpOnly; SameSite=Strict; Max-Age=${SESSION_DURATION}`
                    },
                    {
                        key: 'Set-Cookie',
                        value: `${STATE_COOKIE}=; Domain=${COOKIE_DOMAIN}; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT`
                    },
                    {
                        key: 'Set-Cookie',
                        value: `mc-email-original-url=; Domain=${COOKIE_DOMAIN}; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT`
                    }
                ]
            }
        };
        
    } catch (error) {
        console.error('Token exchange error:', error);
        return createErrorResponse(500, 'Authentication failed');
    }
}

function handleLogout() {
    return {
        status: '302',
        statusDescription: 'Found',
        headers: {
            location: [{
                key: 'Location',
                value: `https://${IDENTITY_CENTER_DOMAIN}/logout`
            }],
            'set-cookie': [{
                key: 'Set-Cookie',
                value: `${SESSION_COOKIE}=; Domain=${COOKIE_DOMAIN}; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT`
            }]
        }
    };
}

function redirectToIdentityCenter(request) {
    // Generate state parameter for CSRF protection
    const state = crypto.randomBytes(32).toString('hex');
    
    // Store original URL
    const originalUrl = `https://${request.headers.host[0].value}${request.uri}${request.querystring ? '?' + request.querystring : ''}`;
    
    // Build authorization URL
    const authParams = {
        response_type: 'code',
        client_id: 'your-client-id', // This should be configured
        redirect_uri: REDIRECT_URI,
        scope: 'openid profile email',
        state: state
    };
    
    const authUrl = `https://${IDENTITY_CENTER_DOMAIN}/oauth2/authorize?${querystring.stringify(authParams)}`;
    
    return {
        status: '302',
        statusDescription: 'Found',
        headers: {
            location: [{
                key: 'Location',
                value: authUrl
            }],
            'set-cookie': [
                {
                    key: 'Set-Cookie',
                    value: `${STATE_COOKIE}=${state}; Domain=${COOKIE_DOMAIN}; Path=/; Secure; HttpOnly; SameSite=Strict; Max-Age=600`
                },
                {
                    key: 'Set-Cookie',
                    value: `mc-email-original-url=${encodeURIComponent(originalUrl)}; Domain=${COOKIE_DOMAIN}; Path=/; Secure; HttpOnly; SameSite=Strict; Max-Age=600`
                }
            ]
        }
    };
}

async function exchangeCodeForTokens(code) {
    // This would make an HTTP request to Identity Center token endpoint
    // For now, return mock data
    return {
        access_token: 'mock-access-token',
        id_token: 'mock-id-token',
        refresh_token: 'mock-refresh-token'
    };
}

async function validateUserAndGetGroups(accessToken) {
    // This would make requests to Identity Center APIs to get user info and groups
    // For now, return mock data
    return {
        userId: 'user123',
        email: 'user@example.com',
        groups: ['ChangeManagers', 'CustomerManagers-HTS']
    };
}

function isUserAuthorized(userGroups) {
    if (ALLOWED_GROUPS.length === 0) {
        return true; // No restrictions
    }
    
    return userGroups.some(group => ALLOWED_GROUPS.includes(group));
}

function extractCustomerCodes(groups) {
    const customerCodes = [];
    
    groups.forEach(group => {
        // Extract customer codes from group names like "CustomerManagers-HTS"
        const match = group.match(/CustomerManagers-(.+)/);
        if (match) {
            customerCodes.push(match[1].toLowerCase());
        }
    });
    
    return customerCodes;
}

function createSessionToken(sessionData) {
    // In production, this should be a signed JWT or encrypted token
    return Buffer.from(JSON.stringify(sessionData)).toString('base64');
}

async function isValidSession(sessionToken) {
    try {
        const sessionData = JSON.parse(Buffer.from(sessionToken, 'base64').toString());
        return sessionData.expiresAt > Date.now();
    } catch (error) {
        return false;
    }
}

function getCookie(headers, cookieName) {
    const cookieHeader = headers.cookie;
    if (!cookieHeader || !cookieHeader[0]) {
        return null;
    }
    
    const cookies = cookieHeader[0].value.split(';');
    for (const cookie of cookies) {
        const [name, value] = cookie.trim().split('=');
        if (name === cookieName) {
            return decodeURIComponent(value);
        }
    }
    
    return null;
}

function createErrorResponse(statusCode, message) {
    return {
        status: statusCode.toString(),
        statusDescription: message,
        headers: {
            'content-type': [{
                key: 'Content-Type',
                value: 'text/html'
            }]
        },
        body: `
            <!DOCTYPE html>
            <html>
            <head>
                <title>Authentication Error</title>
                <style>
                    body { font-family: Arial, sans-serif; margin: 40px; }
                    .error { color: #d32f2f; }
                </style>
            </head>
            <body>
                <h1 class="error">Authentication Error</h1>
                <p>${message}</p>
                <p><a href="/">Return to Home</a></p>
            </body>
            </html>
        `
    };
}