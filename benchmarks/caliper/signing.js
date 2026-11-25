'use strict';
const crypto = require('node:crypto');

// DIS-request-v1: Go struct field order, omitempty, and encoding/json escaping.
const parameterFields = ['field', 'new_content', 'identity_id',
    'public_key', 'kind', 'data_id', 'address', 'data_signature', 'profile',
    'recipient', 'permissions', 'expires_at', 'status'];
function signingBytes(request) {
    const parameters = {};
    for (const key of Object.keys(request.parameters || {})) {
        if (!parameterFields.includes(key)) throw new Error('Unknown parameter: ' + key);
    }
    for (const field of parameterFields) {
        const value = (request.parameters || {})[field];
        if (value === undefined || value === null || value === '' || value === 0 ||
            (Array.isArray(value) && value.length === 0)) continue;
        if (field === 'profile') {
            parameters.profile = {label: value.label || '', format: value.format || '', description: value.description || ''};
        } else parameters[field] = value;
    }
    for (const value of [request.timestamp, parameters.expires_at]) {
        if (value !== undefined && !Number.isSafeInteger(value)) throw new Error('Timestamps must be safe integers');
    }
    const canonical = {domain: request.domain, operation: request.operation, actor: request.actor,
        request_id: request.request_id, timestamp: request.timestamp, parameters, signature: ''};
    const json = JSON.stringify(canonical).replace(/[<>&\u2028\u2029]/g,
        character => String.fromCharCode(92) + 'u' + character.charCodeAt(0).toString(16).padStart(4, '0'));
    return Buffer.from('DIS-request-v1\n' + json, 'utf8');
}
function signedRequest(domain, operation, actor, requestID, parameters, key) {
    const request = {domain, operation, actor, request_id: requestID,
        timestamp: Math.floor(Date.now() / 1000), parameters, signature: ''};
    request.signature = crypto.sign(null, signingBytes(request), key).toString('base64');
    return JSON.stringify(request);
}
module.exports = {signingBytes, signedRequest};
