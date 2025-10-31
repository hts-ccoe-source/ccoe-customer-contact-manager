'use strict';

import samlify from 'samlify';
import validator from '@authenio/samlify-node-xmllint';
import { toRFC3339, parseDateTime } from './datetime/index.js';

// Set the XML validator
samlify.setSchemaValidator(validator);

// Session timeout configuration
const SESSION_CONFIG = {
  // Idle timeout: session expires after this period of inactivity
  IDLE_TIMEOUT_MS: parseInt(process.env.SESSION_IDLE_TIMEOUT_MS || '10800000'),        // 3 hours (10800000ms)
  
  // Absolute maximum: session expires after this period regardless of activity
  ABSOLUTE_MAX_MS: parseInt(process.env.SESSION_ABSOLUTE_MAX_MS || '43200000'),       // 12 hours (43200000ms)
  
  // Refresh threshold: issue new cookie when this much time remains
  REFRESH_THRESHOLD_MS: parseInt(process.env.SESSION_REFRESH_THRESHOLD_MS || '600000'),       // 10 minutes (600000ms)
  
  // Cookie Max-Age for browser (should match idle timeout)
  COOKIE_MAX_AGE_SECONDS: parseInt(process.env.SESSION_COOKIE_MAX_AGE || '10800')                // 3 hours
};

/**
 * Validates session configuration at startup
 * Ensures all timeout values are sensible and properly ordered
 * @throws {Error} If configuration is invalid
 */
function validateSessionConfig() {
  if (SESSION_CONFIG.IDLE_TIMEOUT_MS <= 0) {
    throw new Error('SESSION_IDLE_TIMEOUT_MS must be positive');
  }
  
  if (SESSION_CONFIG.ABSOLUTE_MAX_MS <= 0) {
    throw new Error('SESSION_ABSOLUTE_MAX_MS must be positive');
  }
  
  if (SESSION_CONFIG.ABSOLUTE_MAX_MS <= SESSION_CONFIG.IDLE_TIMEOUT_MS) {
    throw new Error('SESSION_ABSOLUTE_MAX_MS must be greater than SESSION_IDLE_TIMEOUT_MS');
  }
  
  if (SESSION_CONFIG.REFRESH_THRESHOLD_MS >= SESSION_CONFIG.IDLE_TIMEOUT_MS) {
    throw new Error('SESSION_REFRESH_THRESHOLD_MS must be less than SESSION_IDLE_TIMEOUT_MS');
  }
  
  if (SESSION_CONFIG.REFRESH_THRESHOLD_MS <= 0) {
    throw new Error('SESSION_REFRESH_THRESHOLD_MS must be positive');
  }
  
  if (SESSION_CONFIG.COOKIE_MAX_AGE_SECONDS <= 0) {
    throw new Error('SESSION_COOKIE_MAX_AGE must be positive');
  }
  
  console.log('✅ Session configuration validated');
}

/**
 * Logs the active session configuration at startup
 * Provides visibility into timeout values for monitoring and troubleshooting
 */
function logSessionConfig() {
  console.log('=== SESSION CONFIGURATION ===');
  console.log('Session configuration:', {
    idleTimeout: `${SESSION_CONFIG.IDLE_TIMEOUT_MS / 60000} minutes (${SESSION_CONFIG.IDLE_TIMEOUT_MS / 3600000} hours)`,
    absoluteMax: `${SESSION_CONFIG.ABSOLUTE_MAX_MS / 60000} minutes (${SESSION_CONFIG.ABSOLUTE_MAX_MS / 3600000} hours)`,
    refreshThreshold: `${SESSION_CONFIG.REFRESH_THRESHOLD_MS / 60000} minutes`,
    cookieMaxAge: `${SESSION_CONFIG.COOKIE_MAX_AGE_SECONDS} seconds (${SESSION_CONFIG.COOKIE_MAX_AGE_SECONDS / 3600} hours)`
  });
  console.log('Environment overrides:', {
    SESSION_IDLE_TIMEOUT_MS: process.env.SESSION_IDLE_TIMEOUT_MS || 'not set (using default)',
    SESSION_ABSOLUTE_MAX_MS: process.env.SESSION_ABSOLUTE_MAX_MS || 'not set (using default)',
    SESSION_REFRESH_THRESHOLD_MS: process.env.SESSION_REFRESH_THRESHOLD_MS || 'not set (using default)',
    SESSION_COOKIE_MAX_AGE: process.env.SESSION_COOKIE_MAX_AGE || 'not set (using default)'
  });
  console.log('=============================');
}

// Validate and log configuration at module load time
try {
  validateSessionConfig();
  logSessionConfig();
} catch (error) {
  console.error('❌ Session configuration validation failed:', error.message);
  throw error;
}

// Identity Center SAML metadata
const idpMetadata = `<?xml version="1.0" encoding="UTF-8"?><md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://portal.sso.us-east-1.amazonaws.com/saml/assertion/NzQ4OTA2OTEyNDY5X2lucy00NGQ2M2ZjOGM2OWUyNGJl">
  <md:IDPSSODescriptor WantAuthnRequestsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:KeyDescriptor use="signing">
      <ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">
        <ds:X509Data>
          <ds:X509Certificate>MIIDBzCCAe+gAwIBAgIFAPNCQwMwDQYJKoZIhvcNAQELBQAwRTEWMBQGA1UEAwwNYW1hem9uYXdzLmNvbTENMAsGA1UECwwESURBUzEPMA0GA1UECgwGQW1hem9uMQswCQYDVQQGEwJVUzAeFw0yNTA5MjMxOTM2MzBaFw0zMDA5MjMxOTM2MzBaMEUxFjAUBgNVBAMMDWFtYXpvbmF3cy5jb20xDTALBgNVBAsMBElEQVMxDzANBgNVBAoMBkFtYXpvbjELMAkGA1UEBhMCVVMwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDA0YN/sXuLuXwqRRQpAp0DAK5Cxk09U7RGc2qfiMnwg60/BTBuuRKwG2jTbGkn2SIwOsKiGgrmGokKg1J1XE9Q/MgsxMXZw3Z+cXWRtEBbwxWHhRrHgJyL1McciqDFA8nBupFy25UaXGZCKhPzxOeK7rJxqeI2dHXS+D8uMRdLR6+DCBFo7vDWo5o/gYyBwcfgpnuZsG4WJO1j2fXZPvVSolB3BT7JPM0OXeaa6u2BpBrtqlGXWgHwqTWNXNUi71DC+ZdMaRtIZSSGYHH7ljmF9JBgGDptJMXYCtkXStzcB/PLnyq2Y82bFpF1U+y4Nh4YXUI6Vlm0E/+402z7CoHzAgMBAAEwDQYJKoZIhvcNAQELBQADggEBABgowgW3qiEiIxxDU58U8ZyvSRrRJE8Rqj7wid2OT5odk3p58GDDZe+ad8FsnK9aIuseROqrTDLFVnxNgpAaH5wkiRl3TfLFlxj7sHfx29SAHwl9yNDy0/cIDaulujM5qLnOBnlY4jeFz7I9s5SwHiYW/OVAzElXMxolBrgaCDz7wzzgjpkfZRtc7cAlPUdlsb58hd8O3GORzTf7q1zgTkGVjLpJrxweWq2u/rjMcdRWtrTGa7bQ6GI1YUl3NW93W6VW/76u0xMwcTPhvOLJD0MjzskMQYkOoBMBArj5vw/OMt44qdjc5fxAgWqUT1m3c39GqamON/gLLQHetbP/sB4=</ds:X509Certificate>
        </ds:X509Data>
      </ds:KeyInfo>
    </md:KeyDescriptor>
    <md:SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://portal.sso.us-east-1.amazonaws.com/saml/logout/NzQ4OTA2OTEyNDY5X2lucy00NGQ2M2ZjOGM2OWUyNGJl"/>
    <md:SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://portal.sso.us-east-1.amazonaws.com/saml/logout/NzQ4OTA2OTEyNDY5X2lucy00NGQ2M2ZjOGM2OWUyNGJl"/>
    <md:NameIDFormat/>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://portal.sso.us-east-1.amazonaws.com/saml/assertion/NzQ4OTA2OTEyNDY5X2lucy00NGQ2M2ZjOGM2OWUyNGJl"/>
    <md:SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="https://portal.sso.us-east-1.amazonaws.com/saml/assertion/NzQ4OTA2OTEyNDY5X2lucy00NGQ2M2ZjOGM2OWUyNGJl"/>
  </md:IDPSSODescriptor>
</md:EntityDescriptor>`;

// Service Provider metadata
const spMetadata = `<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://change-management.ccoe.hearst.com">
  <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="false" protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
    <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://change-management.ccoe.hearst.com/saml/acs" index="0" isDefault="true"/>
  </md:SPSSODescriptor>
</md:EntityDescriptor>`;

function parseCookies(cookieHeader) {
    const cookies = {};
    if (!cookieHeader) return cookies;

    cookieHeader.split(';').forEach(cookie => {
        const [name, value] = cookie.trim().split('=');
        if (name && value) {
            cookies[name] = decodeURIComponent(value);
        }
    });
    return cookies;
}

function isAuthorizedUser(userInfo) {
    if (!userInfo || !userInfo.email) {
        return false;
    }

    const normalizedEmail = userInfo.email.toLowerCase();

    // Must be from hearst.com domain
    if (!normalizedEmail.endsWith('@hearst.com')) {
        console.log('User not from hearst.com domain:', userInfo.email);
        return false;
    }

    return true;
}

/**
 * Migrates legacy session data to include lastActivityAt field
 * Handles backward compatibility for sessions created before the refresh feature
 * @param {Object} sessionData - Session data that may be missing lastActivityAt
 * @returns {Object} Migrated session data with lastActivityAt field
 */
function migrateSessionData(sessionData) {
  // If lastActivityAt is missing, use createdAt as the initial value
  if (!sessionData.lastActivityAt && sessionData.createdAt) {
    sessionData.lastActivityAt = sessionData.createdAt;
    console.log('📦 Migrated legacy session for:', sessionData.email);
  }
  return sessionData;
}

/**
 * Validates a session and determines if refresh is needed
 * Implements dual timeout checks: idle timeout and absolute maximum duration
 * @param {Object} sessionData - Decoded session data from cookie
 * @param {number} currentTime - Current timestamp in milliseconds
 * @returns {Object} Validation result with status and refresh flag
 */
function validateSession(sessionData, currentTime) {
  // Validate required fields
  if (!sessionData || !sessionData.email || !sessionData.createdAt || !sessionData.lastActivityAt) {
    return {
      valid: false,
      shouldRefresh: false,
      reason: 'Missing required session fields',
      sessionAge: 0,
      idleTime: 0
    };
  }

  try {
    // Parse timestamps using existing datetime utilities
    const createdDate = parseDateTime(sessionData.createdAt);
    const lastActivityDate = parseDateTime(sessionData.lastActivityAt);
    
    // Calculate time deltas
    const sessionAge = currentTime - createdDate.getTime();
    const idleTime = currentTime - lastActivityDate.getTime();
    
    // Check absolute maximum duration
    if (sessionAge >= SESSION_CONFIG.ABSOLUTE_MAX_MS) {
      return {
        valid: false,
        shouldRefresh: false,
        reason: 'Session exceeded absolute maximum duration',
        sessionAge,
        idleTime
      };
    }
    
    // Check idle timeout
    if (idleTime >= SESSION_CONFIG.IDLE_TIMEOUT_MS) {
      return {
        valid: false,
        shouldRefresh: false,
        reason: 'Session exceeded idle timeout',
        sessionAge,
        idleTime
      };
    }
    
    // Session is valid - determine if refresh is needed
    const timeUntilIdleExpiry = SESSION_CONFIG.IDLE_TIMEOUT_MS - idleTime;
    const shouldRefresh = timeUntilIdleExpiry <= SESSION_CONFIG.REFRESH_THRESHOLD_MS;
    
    return {
      valid: true,
      shouldRefresh,
      reason: shouldRefresh ? 'Session valid, refresh recommended' : 'Session valid',
      sessionAge,
      idleTime
    };
    
  } catch (error) {
    return {
      valid: false,
      shouldRefresh: false,
      reason: `Failed to parse session timestamps: ${error.message}`,
      sessionAge: 0,
      idleTime: 0
    };
  }
}

/**
 * Creates a refreshed session cookie with updated lastActivityAt
 * Preserves the original createdAt timestamp to enforce absolute maximum duration
 * @param {Object} sessionData - Current session data with email, createdAt, and lastActivityAt
 * @returns {Object} Set-Cookie header object with refreshed session data
 */
function refreshSessionCookie(sessionData) {
  // Preserve original createdAt timestamp
  const refreshedSessionData = {
    email: sessionData.email,
    createdAt: sessionData.createdAt,  // Keep original authentication time
    lastActivityAt: toRFC3339(new Date())  // Update to current time
  };

  // Encode session data as base64 JSON
  const sessionValue = Buffer.from(JSON.stringify(refreshedSessionData)).toString('base64');

  // Return Set-Cookie header with proper attributes
  return {
    key: 'Set-Cookie',
    value: `SAML_SESSION=${sessionValue}; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=${SESSION_CONFIG.COOKIE_MAX_AGE_SECONDS}`
  };
}

/**
 * Creates a session cookie for initial authentication or refresh
 * @param {Object} userInfo - User information containing email
 * @param {boolean} isRefresh - Whether this is a refresh operation (default: false)
 * @param {string} existingCreatedAt - Existing createdAt timestamp for refresh operations (optional)
 * @returns {Object} Set-Cookie header object with session data
 */
function createSessionCookie(userInfo, isRefresh = false, existingCreatedAt = null) {
  const now = toRFC3339(new Date());
  
  const sessionData = {
    email: userInfo.email,
    createdAt: existingCreatedAt || now,  // Preserve original on refresh, use current for new sessions
    lastActivityAt: now  // Always update to current time
  };

  // Encode session data as base64 JSON
  const sessionValue = Buffer.from(JSON.stringify(sessionData)).toString('base64');

  // Return Set-Cookie header with proper attributes
  return {
    key: 'Set-Cookie',
    value: `SAML_SESSION=${sessionValue}; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=${SESSION_CONFIG.COOKIE_MAX_AGE_SECONDS}`
  };
}

export const handler = async (event) => {
    try {
        const request = event.Records[0].cf.request;
        const headers = request.headers;
        const uri = request.uri;
        const method = request.method;

        console.log('=== LAMBDA@EDGE SAMLIFY REQUEST START ===');
        console.log('URI:', uri);
        console.log('Method:', method);
        console.log('Headers:', Object.keys(headers));

        // Initialize SAML providers
        const idp = samlify.IdentityProvider({ metadata: idpMetadata });
        const sp = samlify.ServiceProvider({ metadata: spMetadata });

        // Handle SAML ACS endpoint (POST from Identity Center)
        if (uri === '/saml/acs' && method === 'POST') {
            console.log('=== PROCESSING SAML RESPONSE ===');

            let userInfo = null; // Declare userInfo at the proper scope

            try {
                // Parse the SAML response from POST body
                const body = request.body;
                let samlResponse = null;

                if (body && body.data) {
                    const bodyData = Buffer.from(body.data, body.encoding || 'base64').toString();
                    console.log('POST Body:', bodyData);

                    // Parse form data to extract SAMLResponse
                    const params = new URLSearchParams(bodyData);
                    samlResponse = params.get('SAMLResponse');
                }

                if (!samlResponse) {
                    console.error('No SAMLResponse found in POST body');
                    return {
                        status: '400',
                        statusDescription: 'Bad Request',
                        body: 'Missing SAMLResponse'
                    };
                }

                console.log('SAMLResponse received, length:', samlResponse.length);

                // Quick decode and extract email with robust parsing
                const decodedSaml = Buffer.from(samlResponse, 'base64').toString('utf8');
                console.log('SAML response decoded, length:', decodedSaml.length);

                // Extract email using multiple approaches for robustness
                let email = null;

                // Try standard NameID extraction
                const nameIdMatch = decodedSaml.match(/<saml2:NameID[^>]*>([^<]+)<\/saml2:NameID>/);
                if (nameIdMatch && nameIdMatch[1]) {
                    email = nameIdMatch[1].trim();
                }

                // If that fails, try a more flexible approach
                if (!email) {
                    const flexibleMatch = decodedSaml.match(/NameID[^>]*>([^<]*@[^<]*)</);
                    if (flexibleMatch && flexibleMatch[1]) {
                        email = flexibleMatch[1].trim();
                    }
                }

                if (!email || !email.includes('@')) {
                    console.error('Could not extract valid email from SAML response');
                    console.log('First 1000 chars of SAML:', decodedSaml.substring(0, 1000));
                    return {
                        status: '400',
                        statusDescription: 'Bad Request',
                        body: 'Invalid SAML response - no email found'
                    };
                }

                console.log('✅ Extracted email:', email);

                userInfo = {
                    email: email,
                    attributes: {}
                };

                // Check authorization
                if (!isAuthorizedUser(userInfo)) {
                    console.log('❌ User not authorized:', userInfo.email);
                    return {
                        status: '403',
                        statusDescription: 'Forbidden',
                        body: 'Access denied'
                    };
                }

                console.log('✅ User authorized:', userInfo.email);

            } catch (error) {
                console.error('SAML processing failed:', error.message);
                return {
                    status: '500',
                    statusDescription: 'Internal Server Error',
                    body: 'SAML processing failed: ' + error.message
                };
            }

            // Create session and redirect to original URL (userInfo is now accessible here)
            if (!userInfo) {
                console.error('UserInfo is null after SAML processing');
                return {
                    status: '500',
                    statusDescription: 'Internal Server Error',
                    body: 'Failed to extract user information'
                };
            }

            const sessionCookie = createSessionCookie(userInfo);
            const relayState = 'https://change-management.ccoe.hearst.com';

            return {
                status: '302',
                statusDescription: 'Found',
                headers: {
                    location: [{
                        key: 'Location',
                        value: relayState
                    }],
                    'set-cookie': [sessionCookie],
                    'cache-control': [{
                        key: 'Cache-Control',
                        value: 'no-cache, no-store, must-revalidate'
                    }]
                }
            };
        }

        // Handle auth-check endpoint - lightweight authentication verification
        if (uri === '/auth-check') {
            console.log('=== PROCESSING AUTH CHECK ===');
            
            let sessionValid = false;
            let userEmail = null;

            if (headers.cookie) {
                const cookies = parseCookies(headers.cookie[0].value);
                const samlSession = cookies['SAML_SESSION'];

                if (samlSession) {
                    try {
                        // Attempt to decode and parse session cookie
                        let sessionData;
                        try {
                            const decodedSession = Buffer.from(samlSession, 'base64').toString();
                            sessionData = JSON.parse(decodedSession);
                        } catch (parseError) {
                            // Handle JSON parse errors gracefully
                            console.error('❌ Session cookie parse error:', {
                                error: parseError.message,
                                errorType: parseError.name,
                                cookieLength: samlSession.length,
                                timestamp: new Date().toISOString()
                            });
                            // Session is invalid, will redirect to IdP below
                            sessionData = null;
                        }
                        
                        if (sessionData) {
                            // Migrate legacy session data if needed
                            sessionData = migrateSessionData(sessionData);
                            
                            // Validate session using dual timeout checks (same logic as main handler)
                            let validation;
                            try {
                                validation = validateSession(sessionData, Date.now());
                            } catch (validationError) {
                                // Handle timestamp parsing errors with clear error messages
                                console.error('❌ Session validation error:', {
                                    error: validationError.message,
                                    errorType: validationError.name,
                                    email: sessionData.email || 'unknown',
                                    createdAt: sessionData.createdAt || 'missing',
                                    lastActivityAt: sessionData.lastActivityAt || 'missing',
                                    timestamp: new Date().toISOString()
                                });
                                validation = {
                                    valid: false,
                                    shouldRefresh: false,
                                    reason: `Validation error: ${validationError.message}`,
                                    sessionAge: 0,
                                    idleTime: 0
                                };
                            }
                            
                            if (validation.valid) {
                                sessionValid = true;
                                userEmail = sessionData.email;
                                
                                // Log validation results for monitoring
                                console.log('✅ Auth check passed for user:', userEmail);
                                console.log(`📊 Session metrics - Age: ${Math.floor(validation.sessionAge / 60000)}min, Idle: ${Math.floor(validation.idleTime / 60000)}min`);
                            } else {
                                // Log all session validation failures with details
                                console.error('❌ Auth check failed - session validation failed:', {
                                    email: sessionData.email || 'unknown',
                                    reason: validation.reason,
                                    sessionAge: `${Math.floor(validation.sessionAge / 60000)}min`,
                                    idleTime: `${Math.floor(validation.idleTime / 60000)}min`,
                                    createdAt: sessionData.createdAt || 'missing',
                                    lastActivityAt: sessionData.lastActivityAt || 'missing',
                                    timestamp: new Date().toISOString()
                                });
                            }
                        }
                    } catch (error) {
                        // Catch any unexpected errors during session processing
                        console.error('❌ Unexpected error during auth-check session processing:', {
                            error: error.message,
                            errorType: error.name,
                            stack: error.stack,
                            timestamp: new Date().toISOString()
                        });
                        // Session is invalid, will redirect to IdP below
                    }
                }
            }

            if (sessionValid) {
                // Return 200 with authenticated status for valid sessions
                return {
                    status: '200',
                    statusDescription: 'OK',
                    headers: {
                        'content-type': [{
                            key: 'Content-Type',
                            value: 'application/json'
                        }],
                        'cache-control': [{
                            key: 'Cache-Control',
                            value: 'no-cache, no-store, must-revalidate'
                        }]
                    },
                    body: JSON.stringify({
                        authenticated: true,
                        user: userEmail,
                        message: 'User is authenticated'
                    })
                };
            } else {
                // Redirect to IdP on any validation error
                console.log('❌ Auth check failed - redirecting to IdP');
                try {
                    const { context: loginRequestUrl } = sp.createLoginRequest(idp, 'redirect');
                    
                    return {
                        status: '302',
                        statusDescription: 'Found',
                        headers: {
                            location: [{
                                key: 'Location',
                                value: loginRequestUrl
                            }],
                            'cache-control': [{
                                key: 'Cache-Control',
                                value: 'no-cache, no-store, must-revalidate'
                            }]
                        }
                    };
                } catch (error) {
                    console.error('❌ Error creating SAML AuthnRequest for auth-check:', {
                        error: error.message,
                        errorType: error.name,
                        timestamp: new Date().toISOString()
                    });
                    return {
                        status: '500',
                        statusDescription: 'Internal Server Error',
                        body: 'Authentication service error'
                    };
                }
            }
        }

        // Check for existing session for all other requests
        console.log('=== SESSION VALIDATION ===');
        let sessionValid = false;
        let userInfo = null;

        if (headers.cookie) {
            console.log('Cookies found in request');
            const cookies = parseCookies(headers.cookie[0].value);
            const samlSession = cookies['SAML_SESSION'];

            if (samlSession) {
                try {
                    // Wrap session validation in try-catch blocks
                    let sessionData;
                    
                    // Handle JSON parse errors gracefully
                    try {
                        const decodedSession = Buffer.from(samlSession, 'base64').toString();
                        sessionData = JSON.parse(decodedSession);
                    } catch (parseError) {
                        // Log all session validation failures with details
                        console.error('❌ Session cookie parse error:', {
                            error: parseError.message,
                            errorType: parseError.name,
                            cookieLength: samlSession.length,
                            timestamp: new Date().toISOString()
                        });
                        // Session is invalid, will redirect to IdP below
                        sessionData = null;
                    }
                    
                    if (sessionData) {
                        // Migrate legacy session data if needed
                        try {
                            sessionData = migrateSessionData(sessionData);
                        } catch (migrationError) {
                            console.error('❌ Session migration error:', {
                                error: migrationError.message,
                                errorType: migrationError.name,
                                email: sessionData.email || 'unknown',
                                timestamp: new Date().toISOString()
                            });
                            // Continue with unmigrated data - validation will catch issues
                        }
                        
                        // Validate session using dual timeout checks
                        let validation;
                        try {
                            validation = validateSession(sessionData, Date.now());
                        } catch (validationError) {
                            // Handle timestamp parsing errors with clear error messages
                            console.error('❌ Session validation error:', {
                                error: validationError.message,
                                errorType: validationError.name,
                                email: sessionData.email || 'unknown',
                                createdAt: sessionData.createdAt || 'missing',
                                lastActivityAt: sessionData.lastActivityAt || 'missing',
                                timestamp: new Date().toISOString()
                            });
                            validation = {
                                valid: false,
                                shouldRefresh: false,
                                reason: `Validation error: ${validationError.message}`,
                                sessionAge: 0,
                                idleTime: 0
                            };
                        }
                        
                        if (validation.valid) {
                            // Log session validation success with metrics
                            console.log('✅ Valid session found for user:', sessionData.email);
                            console.log(`📊 Session metrics - Age: ${Math.floor(validation.sessionAge / 60000)}min, Idle: ${Math.floor(validation.idleTime / 60000)}min`);
                            
                            sessionValid = true;
                            userInfo = sessionData;
                            
                            // Check if session refresh is needed
                            if (validation.shouldRefresh) {
                                console.log('🔄 Session refresh triggered for user:', sessionData.email);
                                console.log(`📊 Refresh metrics - Previous activity: ${sessionData.lastActivityAt}, Time until idle expiry: ${Math.floor((SESSION_CONFIG.IDLE_TIMEOUT_MS - validation.idleTime) / 60000)}min`);
                                
                                // Continue without refresh if refresh cookie creation fails
                                try {
                                    // Generate refreshed cookie
                                    const refreshedCookie = refreshSessionCookie(sessionData);
                                    
                                    // Return 204 No Content response with refreshed cookie
                                    // Browser will update cookie and automatically retry the request
                                    console.log('✅ Session refresh successful, returning 204 with new cookie');
                                    return {
                                        status: '204',
                                        statusDescription: 'No Content',
                                        headers: {
                                            'set-cookie': [refreshedCookie],
                                            'cache-control': [{
                                                key: 'Cache-Control',
                                                value: 'no-cache, no-store, must-revalidate'
                                            }]
                                        }
                                    };
                                } catch (refreshError) {
                                    // Continue without refresh if refresh cookie creation fails
                                    console.error('⚠️ Session refresh failed, continuing without refresh:', {
                                        error: refreshError.message,
                                        errorType: refreshError.name,
                                        email: sessionData.email || 'unknown',
                                        timestamp: new Date().toISOString()
                                    });
                                    // Session is still valid, allow request to proceed
                                }
                            }
                        } else {
                            // Log all session validation failures with details
                            console.error('❌ Session validation failed:', {
                                email: sessionData.email || 'unknown',
                                reason: validation.reason,
                                sessionAge: `${Math.floor(validation.sessionAge / 60000)}min`,
                                idleTime: `${Math.floor(validation.idleTime / 60000)}min`,
                                createdAt: sessionData.createdAt || 'missing',
                                lastActivityAt: sessionData.lastActivityAt || 'missing',
                                timestamp: new Date().toISOString()
                            });
                        }
                    }
                } catch (error) {
                    // Catch any unexpected errors during session processing
                    console.error('❌ Unexpected error during session validation:', {
                        error: error.message,
                        errorType: error.name,
                        stack: error.stack,
                        timestamp: new Date().toISOString()
                    });
                    // Session is invalid, will redirect to IdP below
                }
            }
        } else {
            console.log('No cookies found in request');
        }

        // Redirect to IdP on any validation error
        if (!sessionValid) {
            console.log('❌ No valid session found, initiating SAML authentication');

            try {
                // Generate SAML AuthnRequest
                const { context: loginRequestUrl } = sp.createLoginRequest(idp, 'redirect');

                console.log('Generated SAML AuthnRequest URL:', loginRequestUrl);
                console.log('=== REDIRECTING TO IDENTITY CENTER ===');

                return {
                    status: '302',
                    statusDescription: 'Found',
                    headers: {
                        location: [{
                            key: 'Location',
                            value: loginRequestUrl
                        }],
                        'cache-control': [{
                            key: 'Cache-Control',
                            value: 'no-cache, no-store, must-revalidate'
                        }]
                    }
                };

            } catch (error) {
                console.error('❌ Error creating SAML AuthnRequest:', {
                    error: error.message,
                    errorType: error.name,
                    timestamp: new Date().toISOString()
                });
                return {
                    status: '500',
                    statusDescription: 'Internal Server Error',
                    body: 'Authentication error: ' + error.message
                };
            }
        }

        // Valid session - add user info to headers and allow request
        console.log('✅ User authenticated successfully:', userInfo.email);
        request.headers['x-user-email'] = [{ key: 'X-User-Email', value: userInfo.email || 'unknown' }];
        request.headers['x-user-groups'] = [{ key: 'X-User-Groups', value: userInfo.groups?.join(',') || 'hearst-user' }];
        request.headers['x-authenticated'] = [{ key: 'X-Authenticated', value: 'true' }];

        console.log('=== LAMBDA@EDGE REQUEST END - ALLOWING ACCESS ===');
        return request;

    } catch (error) {
        console.error('Lambda@Edge error:', error);
        return {
            status: '500',
            statusDescription: 'Internal Server Error',
            body: 'Authentication service error: ' + error.message
        };
    }
};