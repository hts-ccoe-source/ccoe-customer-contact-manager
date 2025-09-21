// Lambda@Edge function for adding security headers
// Adds comprehensive security headers to all responses

exports.handler = async (event) => {
    const response = event.Records[0].cf.response;
    const headers = response.headers;
    
    // Content Security Policy
    headers['content-security-policy'] = [{
        key: 'Content-Security-Policy',
        value: process.env.CSP_POLICY || "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' https:; frame-ancestors 'none';"
    }];
    
    // Strict Transport Security
    headers['strict-transport-security'] = [{
        key: 'Strict-Transport-Security',
        value: `max-age=${process.env.HSTS_MAX_AGE || '31536000'}; includeSubDomains; preload`
    }];
    
    // X-Frame-Options
    headers['x-frame-options'] = [{
        key: 'X-Frame-Options',
        value: 'DENY'
    }];
    
    // X-Content-Type-Options
    headers['x-content-type-options'] = [{
        key: 'X-Content-Type-Options',
        value: 'nosniff'
    }];
    
    // X-XSS-Protection
    headers['x-xss-protection'] = [{
        key: 'X-XSS-Protection',
        value: '1; mode=block'
    }];
    
    // Referrer Policy
    headers['referrer-policy'] = [{
        key: 'Referrer-Policy',
        value: 'strict-origin-when-cross-origin'
    }];
    
    // Permissions Policy
    headers['permissions-policy'] = [{
        key: 'Permissions-Policy',
        value: 'camera=(), microphone=(), geolocation=(), payment=(), usb=(), magnetometer=(), gyroscope=(), accelerometer=()'
    }];
    
    // Cross-Origin Embedder Policy
    headers['cross-origin-embedder-policy'] = [{
        key: 'Cross-Origin-Embedder-Policy',
        value: 'require-corp'
    }];
    
    // Cross-Origin Opener Policy
    headers['cross-origin-opener-policy'] = [{
        key: 'Cross-Origin-Opener-Policy',
        value: 'same-origin'
    }];
    
    // Cross-Origin Resource Policy
    headers['cross-origin-resource-policy'] = [{
        key: 'Cross-Origin-Resource-Policy',
        value: 'same-origin'
    }];
    
    // Cache Control for sensitive pages
    if (response.status === '200' && isAuthenticatedPage(event.Records[0].cf.request.uri)) {
        headers['cache-control'] = [{
            key: 'Cache-Control',
            value: 'no-cache, no-store, must-revalidate, private'
        }];
        
        headers['pragma'] = [{
            key: 'Pragma',
            value: 'no-cache'
        }];
        
        headers['expires'] = [{
            key: 'Expires',
            value: '0'
        }];
    }
    
    // Remove server information
    delete headers['server'];
    delete headers['x-powered-by'];
    
    return response;
};

function isAuthenticatedPage(uri) {
    // Pages that require authentication and should not be cached
    const authenticatedPaths = [
        '/dashboard',
        '/upload',
        '/my-changes',
        '/edit-change',
        '/api/'
    ];
    
    return authenticatedPaths.some(path => uri.startsWith(path));
}