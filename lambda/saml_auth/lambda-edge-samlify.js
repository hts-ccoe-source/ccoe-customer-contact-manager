'use strict';

import samlify from 'samlify';
import validator from '@authenio/samlify-node-xmllint';
import { toRFC3339, parseDateTime } from './datetime/index.js';

// Set the XML validator
samlify.setSchemaValidator(validator);

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

function createSessionCookie(userInfo) {
    const sessionData = {
        email: userInfo.email,
        createdAt: toRFC3339(new Date())
    };

    const sessionValue = Buffer.from(JSON.stringify(sessionData)).toString('base64');

    return {
        key: 'Set-Cookie',
        value: `SAML_SESSION=${sessionValue}; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=3600`
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
                        const sessionData = JSON.parse(Buffer.from(samlSession, 'base64').toString());
                        
                        // Validate session age using RFC3339 timestamp
                        const createdDate = parseDateTime(sessionData.createdAt);
                        const sessionAge = Date.now() - createdDate.getTime();

                        if (sessionAge < 3600000) { // 1 hour
                            sessionValid = true;
                            userEmail = sessionData.email;
                        }
                    } catch (error) {
                        console.error('Failed to decode session for auth-check:', error);
                    }
                }
            }

            if (sessionValid) {
                console.log('✅ Auth check passed for user:', userEmail);
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
                console.log('❌ Auth check failed - no valid session');
                // Generate SAML AuthnRequest for auth-check
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
                    console.error('Error creating SAML AuthnRequest for auth-check:', error);
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
                    const sessionData = JSON.parse(Buffer.from(samlSession, 'base64').toString());
                    
                    // Validate session age using RFC3339 timestamp
                    const createdDate = parseDateTime(sessionData.createdAt);
                    const sessionAge = Date.now() - createdDate.getTime();

                    if (sessionAge < 3600000) { // 1 hour
                        console.log('✅ Valid session found for user:', sessionData.email);
                        sessionValid = true;
                        userInfo = sessionData;
                    } else {
                        console.log('❌ Session expired');
                    }
                } catch (error) {
                    console.error('❌ Failed to decode session:', error);
                }
            }
        } else {
            console.log('No cookies found in request');
        }

        // If no valid session, initiate SAML authentication
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
                console.error('Error creating SAML AuthnRequest:', error);
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