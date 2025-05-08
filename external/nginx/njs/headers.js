const userIdHeader = process.env.UID_HTTP_HEADER || 'X-Credential-Identifier';

function userHeader(r) {
    // Extract the username from the request header
    return r.headersIn[userIdHeader];
}

export default {userHeader}