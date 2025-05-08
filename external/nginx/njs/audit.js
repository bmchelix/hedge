let fs = require('fs');

const userIdHeader = process.env.UID_HTTP_HEADER || 'X-Credential-Identifier';
const logFile = '/etc/nginx/njs/logs/internal.log';

function log(r, data, flags) {
    let request_uri = r.uri;
    let request_method = r.method;

    if (request_method !== 'POST' && request_method !== 'PUT' && request_method !== 'DELETE' && request_method !== 'PATCH') {
        ngx.log(ngx.INFO, `skipping logging audit path: '${request_uri}' method: '${request_method}'`);
        return
    }

    let d = new Date();

    let caller_ip = r.remoteAddress;
    let http_user_agent = r.headersIn["User-Agent"];
    let http_referer = r.headersIn["Referer"];
    let http_x_forwarded_for = r.headersIn["X-Forwarded-For"];
    let http_host = r.headersIn["Host"];
    let http_cookie = r.headersIn["Cookie"];
    let http_content_length = r.headersIn["Content-Length"];
    let user_header = r.headersIn[userIdHeader];
    let http_status = r.status
    let http_response_length = r.bytesSent

    ngx.log(ngx.INFO, `logging audit path: '${request_uri}' method: '${request_method}'`);

    let audit = {
        "time": d.toLocaleString(),
        "caller_ip": caller_ip,
        "user_id": user_header,
        "request_method": request_method,
        "request_uri": request_uri,
        "user_agent": http_user_agent,
        "referer": http_referer,
        "x_forwarded_for": http_x_forwarded_for,
        "host": http_host,
        "cookies": http_cookie,
        "content_length": http_content_length,
        "response_status": http_status,
        "response_length": http_response_length
    };

    fs.appendFileSync(logFile, JSON.stringify(audit) + "\n");
}

export default {log}