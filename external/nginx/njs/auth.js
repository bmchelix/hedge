let userSessionCache = new Map();

const userIdHeader = process.env.UID_HTTP_HEADER || 'X-Credential-Identifier';

async function addCSRF(r) {
    const requestMethod = r.variables.request_method;
    const requestedResourceURL = r.variables.request_uri;

    if (requestMethod === 'GET' && requestedResourceURL.includes('/hedge/api/v3/usr_mgmt/user/auth_type')) {
        let token = getCookie(r, 'csrf_token');
        ngx.log(ngx.INFO, `Request intercepted: ${r.variables.request_uri}`);
        ngx.log(ngx.INFO, `CSRF token from cookie: ${token}`);

        try {
            const authTypeResponse = await getAuthTypeFromBE(r);
            ngx.log(ngx.INFO, `Auth_type returned in subrequest: ${JSON.stringify(authTypeResponse)}`);
            if (authTypeResponse === "") {
                ngx.log(ngx.ERR, `Request failed while getting auth type`);
                r.return(502, JSON.stringify({error: "Failed to retrieve authentication type"}));
            } else {
                let response = JSON.stringify({"isExternalAuth": authTypeResponse});
                r.headersOut['Content-Type'] = 'application/json';
                if (!token) {
                    ngx.log(ngx.INFO, "CSRF token not found in cookies. Generating new CSRF token for GET auth_type request...");

                    let csrfToken = generateCSRFToken();
                    ngx.log(ngx.INFO, `Generated new token: ${csrfToken}`);

                    if (authTypeResponse){
                        r.headersOut['Set-Cookie'] = `csrf_token=${csrfToken}; Path=/; Secure; SameSite=Strict`;
                    } else {
                        r.headersOut['Set-Cookie'] = `csrf_token=${csrfToken}; Path=/`;
                    }
                }
                r.return(200, response);
            }
        } catch (error) {
            ngx.log(ngx.ERR, `Error handling auth type request: ${error.message}`);
            r.return(500, JSON.stringify({error: "Internal Server Error"}));
        }
    }
}

async function access(r) {
    ngx.log(ngx.INFO, `Checking Access To ${r.variables.request_method} ${r.variables.request_uri} ....`);

    const requestedResourceURL = r.variables.request_uri;
    const requestMethod = r.variables.request_method;

    // extract user from the request header
    let userName = r.headersIn[userIdHeader];

    // get user from basic auth if header not set
    if (!userName && r.headersIn["Authorization"] !== "") {
        const tmp = Buffer.from(r.headersIn["Authorization"].split(" ")[1], "base64").toString();
        userName = tmp.split(":")[0];
    }

    // check public URL
    if (isPublicUri(requestedResourceURL, userName)) {
        r.return(200);
        return
    }

    if (!userName) {
        r.return(403);
        return
    }

    try {
        const userResourceUriList = await getUserResourceUriList(r, userName);

        if (requestedResourceURL.startsWith('/hedge/api/v3/usr_mgmt/usercontext')) {
            r.return(200);
            return;
        }

        //ngx.log(ngx.INFO, `userResourceUriList for ${requestMethod} ${requestedResourceURL} uri - ${userResourceUriList}`);

        r.headersOut['X-User-Resources'] = userResourceUriList;

        if (!userResourceUriList || userResourceUriList.length === 0 || !userResourceUriList.some(value => requestedResourceURL.startsWith(value))) {
            ngx.log(ngx.ERR, `user resource list not include ${requestedResourceURL}`);

            r.return(403);
            return
        }
    } catch (error) {
        ngx.log(ngx.ERR, `Error processing access: ${error}`);
        r.return(403);
        return
    }

    if (requestMethod === 'POST' || requestMethod === 'PUT' || requestMethod === 'DELETE' || requestMethod === 'PATCH') {
        if (!requestedResourceURL.includes("/rest-device") && !requestedResourceURL.includes("/hedge-grafana")) {
            ngx.log(ngx.INFO, `Going to validate CSRF token for ${requestMethod} ${requestedResourceURL} uri...`);

            if (!validateCSRFToken(r)) {
                ngx.log(ngx.ERR, 'CSRF token validation failed');
                r.return(403, "Invalid CSRF token");
                return;
            }
            ngx.log(ngx.INFO, "CSRF token validation successful.");
        }
    }

    r.return(200);
}

function generateCSRFToken() {
    const characters = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_';
    let token = '';
    for (let i = 0; i < 25; i++) {
        token += characters.charAt(Math.floor(Math.random() * characters.length));
    }
    return token;
}

// Utility to validate CSRF token
function validateCSRFToken(r) {
    const csrfTokenFromCookie = getCookie(r, 'csrf_token');
    const csrfTokenFromHeader = r.headersIn['X-CSRF-Token'];
    ngx.log(ngx.INFO, `Validating CSRF token: Cookie=${csrfTokenFromCookie}, Header=${csrfTokenFromHeader}`);
    if (!csrfTokenFromCookie || !csrfTokenFromHeader) {
        ngx.log(ngx.ERR, 'CSRF token validation failed: One or both tokens are empty');
        return false;
    }
    return csrfTokenFromCookie === csrfTokenFromHeader;
}

// Utility to get a cookie by name
function getCookie(r, name) {
    const cookies = r.variables.http_cookie

    if (cookies) {
        let parts = cookies.split(';');
        for (let i = 0; i < parts.length; i++) {
            let part = parts[i].trim();
            let separatorIndex = part.indexOf('=');
            if (separatorIndex !== -1) {
                let cookieName = part.substring(0, separatorIndex);
                let cookieValue = part.substring(separatorIndex + 1);
                if (cookieName === name) {
                    return cookieValue;
                }
            }
        }
    }
    return null;
}

async function getUserResourceUriList(r, username) {
    if (userSessionCache.has(username)) {
        return Promise.resolve(userSessionCache.get(username));
    }


    return new Promise((resolve, reject) => {
        ngx.log(ngx.INFO, "Calling Resource API...")
        r.subrequest(`/hedge/api/v3/usr_mgmt/user/${username}/resource_url/all`, {method: 'GET'}, (reply) => {
            if (reply.status !== 200) {
                ngx.log(ngx.ERR, `Failed to fetch user resources for ${username}.`);
                resolve([]);
                return;
            }

            const userResourceUriList = JSON.parse(reply.responseText);

            userSessionCache.set(username, userResourceUriList);
            resolve(userResourceUriList);
        });
    });
}

async function getAuthTypeFromBE(r) {
    return new Promise((resolve, reject) => {
        ngx.log(ngx.INFO, "Calling API to get auth_type...")
        r.subrequest(`/hedge/api/v3/usr_mgmt/user/auth_type_sub`, {method: 'GET'}, (reply) => {
            ngx.log(ngx.INFO, `Reply status: ${reply.status}`)

            if (reply.status !== 200) {
                ngx.log(ngx.ERR, `Failed to get auth type from BE.`);
                resolve("");
                return;
            }

            const authType = JSON.parse(reply.responseText);
            resolve(authType);
        });
    });
}


function isPublicUri(requestedResourceURL, userId) {
    return requestedResourceURL.startsWith('/assets') ||
        requestedResourceURL.startsWith('/hedge/signin') ||
        requestedResourceURL.startsWith('/admin') ||
        requestedResourceURL.startsWith('/hedge/api/v3/usr_mgmt/user/pwd') ||
        requestedResourceURL.startsWith('/hedge/api/v3/usr_mgmt/user_preference') ||
        (userId !== '' && requestedResourceURL.startsWith(`/hedge/api/v3/usr_mgmt/user/${userId}/resources`)) ||
        requestedResourceURL.startsWith('/hedge/api/v3/usr_mgmt/user/auth_type') ||
        requestedResourceURL.includes('resource_url/') ;
}

function Map() {
    this.obj = {};
}

Map.prototype.set = function (key, value) {
    this.obj[key] = value;
};

Map.prototype.get = function (key) {
    return this.obj[key];
};

Map.prototype.has = function (key) {
    return this.obj.hasOwnProperty(key);
};

Map.prototype.delete = function (key) {
    delete this.obj[key];
};

Map.prototype.clear = function () {
    this.obj = {};
};

Map.prototype.entries = function () {
    return Object.entries(this.obj);
};

Map.prototype.keys = function () {
    return Object.keys(this.obj);
};

Map.prototype.values = function () {
    return Object.values(this.obj);
};

export default {access, addCSRF}
