#!/usr/bin/dumb-init /bin/sh
#  ----------------------------------------------------------------------------------
#  Copyright (c) 2022-2023 Intel Corporation
#
#  Licensed under the Apache License, Version 2.0 (the "License");
#  you may not use this file except in compliance with the License.
#  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.
#
#  SPDX-License-Identifier: Apache-2.0
#  ----------------------------------------------------------------------------------

set -e

keyfile=nginx.key
certfile=nginx.crt

# This script should run as root as it contains chown commands.
# SKIP_CHOWN is provided in the event that it can be arranged
# that /etc/ssl/nginx is writable by uid/gid 101
# and the container is also started as uid/gid 101.

# Check for default TLS certificate for reverse proxy, create if missing
# Normally we would run the below command in the nginx container itself,
# but nginx:alpine-slim does not include openssl, thus run it here instead.
if test -d /etc/ssl/nginx ; then
    cd /etc/ssl/nginx
    if test ! -f "${keyfile}" ; then
        # (NGINX will restart in a failure loop until a TLS key exists)
        # Create default TLS certificate with 1 day expiry -- user must replace in production (do this as nginx user)
        openssl req -x509 -nodes -days 1 -newkey ec -pkeyopt ec_paramgen_curve:secp384r1 -subj '/CN=localhost/O=EdgeX Foundry' -keyout "${keyfile}" -out "${certfile}" -addext "keyUsage = digitalSignature, keyCertSign" -addext "extendedKeyUsage = serverAuth"
        if [ -z "$SKIP_CHOWN" ]; then
            # nginx process user is 101:101
            chown 101:101 "${keyfile}" "${certfile}"
        fi
        echo "Default TLS certificate created.  Recommend replace with your own."
    fi
fi

#
# Import CORS configuration from common config
#

: ${EDGEX_SERVICE_CORSCONFIGURATION_ENABLECORS:=`yq -r .all-services.Service.CORSConfiguration.EnableCORS /edgex/res/common_configuration.yaml`}
: ${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWCREDENTIALS:=`yq -r .all-services.Service.CORSConfiguration.CORSAllowCredentials /edgex/res/common_configuration.yaml`}
: ${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWEDORIGIN:=`yq -r .all-services.Service.CORSConfiguration.CORSAllowedOrigin /edgex/res/common_configuration.yaml`}
: ${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWEDMETHODS:=`yq -r .all-services.Service.CORSConfiguration.CORSAllowedMethods /edgex/res/common_configuration.yaml`}
: ${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWEDHEADERS:=`yq -r .all-services.Service.CORSConfiguration.CORSAllowedHeaders /edgex/res/common_configuration.yaml`}
: ${EDGEX_SERVICE_CORSCONFIGURATION_CORSEXPOSEHEADERS:=`yq -r .all-services.Service.CORSConfiguration.CORSExposeHeaders /edgex/res/common_configuration.yaml`}
: ${EDGEX_SERVICE_CORSCONFIGURATION_CORSMAXAGE:=`yq -r .all-services.Service.CORSConfiguration.CORSMaxAge /edgex/res/common_configuration.yaml`}

echo "$(date) CORS settings dump ..."
( set | grep EDGEX_SERVICE_CORSCONFIGURATION ) || true

# See https://github.com/edgexfoundry/edgex-go/issues/4648 as to why CORS is implemented this way.
# Warning: no not simplify add_header redundancy. See https://www.peterbe.com/plog/be-very-careful-with-your-add_header-in-nginx
corssnippet=/etc/nginx/templates/cors.block.$$
touch "${corssnippet}"
if test "${EDGEX_SERVICE_CORSCONFIGURATION_ENABLECORS}" = "true"; then
  echo "      if (\$request_method = 'OPTIONS') {" >> "${corssnippet}"
  echo "        add_header 'Access-Control-Allow-Origin' '${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWEDORIGIN}';" >> "${corssnippet}"
  echo "        add_header 'Access-Control-Allow-Methods' '${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWEDMETHODS}';" >> "${corssnippet}"
  echo "        add_header 'Access-Control-Allow-Headers' '${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWEDHEADERS}';" >> "${corssnippet}"
  if test "${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWCREDENTIALS}" = "true"; then
    # CORS specificaiton says that if not true, omit the header entirely
    echo "        add_header 'Access-Control-Allow-Credentials' '${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWCREDENTIALS}';" >> "${corssnippet}"
  fi
  echo "        add_header 'Access-Control-Max-Age' ${EDGEX_SERVICE_CORSCONFIGURATION_CORSMAXAGE};" >> "${corssnippet}"
  echo "        add_header 'Vary' 'origin';" >> "${corssnippet}"
  echo "        add_header 'Content-Type' 'text/plain; charset=utf-8';" >> "${corssnippet}"
  echo "        add_header 'Content-Length' 0;" >> "${corssnippet}"
  echo "        return 204;" >> "${corssnippet}"
  echo "      }" >> "${corssnippet}"
  echo "      if (\$request_method != 'OPTIONS') {" >> "${corssnippet}"
  # Always add headers regardless of response code.  Omit preflight-related headers (allow-methods, allow-headers, allow-credentials, max-age)
  echo "        add_header 'Access-Control-Allow-Origin' '${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWEDORIGIN}' always;" >> "${corssnippet}"
  echo "        add_header 'Access-Control-Expose-Headers' '${EDGEX_SERVICE_CORSCONFIGURATION_CORSEXPOSEHEADERS}' always;" >> "${corssnippet}"
  if test "${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWCREDENTIALS}" = "true"; then
    # CORS specificaiton says that if not true, omit the header entirely
    echo "        add_header 'Access-Control-Allow-Credentials' '${EDGEX_SERVICE_CORSCONFIGURATION_CORSALLOWCREDENTIALS}';" >> "${corssnippet}"
  fi
  echo "        add_header 'Vary' 'origin' always;" >> "${corssnippet}"
  echo "      }" >> "${corssnippet}"
  echo "" >> "${corssnippet}"
fi

#
# Generate NGINX configuration based on EDGEX_ADD_PROXY_ROUTE and standard settings
#

echo "$(date) Generating default NGINX config ..."

# Truncate the template file before we start appending
: >/etc/nginx/templates/generated-routes.inc.template

IFS=', '
for service in ${EDGEX_ADD_PROXY_ROUTE}; do
	prefix=$(echo -n "${service}" | sed -n -e 's/\([-0-9a-zA-Z]*\)\..*/\1/p')
	host=$(echo -n "${service}" | sed -n -e 's/.*\/\/\([-0-9a-zA-Z]*\):.*/\1/p')
	port=$(echo -n "${service}" | sed -n -e 's/.*:\(\d*\)/\1/p')
	varname=$(echo -n "${prefix}" | tr '-' '_')
	echo $service $prefix $host $port
  cat >> /etc/nginx/templates/generated-routes.inc.template <<EOH

set \$upstream_$varname $host;
location /$prefix {
`cat "${corssnippet}"`
  rewrite            /$prefix/(.*) /\$1 break;
  resolver           127.0.0.11 valid=30s;
  proxy_pass         http://\$upstream_$varname:$port;
  proxy_redirect     off;
  proxy_set_header   Host \$host;
  auth_request       /auth;
  auth_request_set   \$auth_status \$upstream_status;
}
EOH

done
unset IFS

# For DEMO profile additional automation route to node-red is enabled
if echo "$COMPOSE_PROFILES" | grep -iq "demo"; then
    RED_NODE_DEMO="
    # Will only enabled if PROFILE=DEMO. Allows for automation to bypass requests to node-red without authentication
    location /automation/workflow {
      resolver           127.0.0.11 valid=30s;
      rewrite            /automation/workflow/(.*) /$1 break;
      proxy_pass         http://hedge-node-red:1880;
      proxy_set_header   Host \$host;
      proxy_set_header   x-userId \$user_header;
      proxy_set_header   X-Consumer-Username \$user_header;
      proxy_set_header   Upgrade \$http_upgrade;
      proxy_set_header   Connection "Upgrade";
      proxy_set_header   X-CSRF-Token \$http_x_csrf_token;
    }
"
else
    RED_NODE_DEMO=''
fi


# This file can be modified by the user; deleted when docker volumes are pruned;
# but preserved across start/up and stop/down actions
if test -f /etc/nginx/templates/edgex-custom-rewrites.inc.template; then
  echo "Using existing custom-rewrites."
else
  cat <<'EOH' > /etc/nginx/templates/edgex-custom-rewrites.inc.template
# Add custom location directives to this file, for example:

# set $upstream_device_virtual edgex-device-virtual;
# location /device-virtual {
#   rewrite            /device-virtual/(.*) /$1 break;
#   resolver           127.0.0.11 valid=30s;
#   proxy_pass         http://$upstream_device_virtual:59900;
#   proxy_redirect     off;
#   proxy_set_header   Host $host;
#   auth_request       /auth;
#   auth_request_set   $auth_status $upstream_status;
# }
EOH
fi

cat <<EOH > /etc/nginx/templates/edgex-default.conf.template
#
# Copyright (C) Intel Corporation 2023
# SPDX-License-Identifier: Apache-2.0
#

# generated 2023-01-19, Mozilla Guideline v5.6, nginx 1.17.7, OpenSSL 1.1.1k, modern configuration, no HSTS, no OCSP
# https://ssl-config.mozilla.org/#server=nginx&version=1.17.7&config=modern&openssl=1.1.1k&hsts=false&ocsp=false&guideline=5.6
server {
    listen 8000;  # Docker deployments only
    #listen 8443 ssl;

    ssl_certificate "/etc/ssl/nginx/nginx.crt";
    ssl_certificate_key "/etc/ssl/nginx/nginx.key";
    ssl_session_tickets off;
    # modern configuration
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers off;

    gzip on;
    gzip_disable "msie6";
    gzip_comp_level 6;
    gzip_min_length 1100;
    gzip_buffers 16 8k;
    gzip_proxied any;
    gzip_types
        text/plain
        text/css
        text/js
        text/xml
        text/javascript
        application/javascript
        application/x-javascript
        application/json
        application/xml
        application/rss+xml
        image/svg+xml/javascript;
    # Logging
    error_log /dev/stderr;
    access_log /dev/stdout;


    js_set \$user_header   headers.userHeader;
    js_set \$external_auth env.getAuth;

    set \$basic_auth_required "Restricted Access";
    if (\$external_auth = "true") {
      set \$basic_auth_required off;
    }

    # external authentication

    location /external/auth {
        internal;
        js_content       auth.access;
    }

    # CSP Settings
    sub_filter_once off;
    sub_filter random_nonce_value \$request_id;
    add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; form-action 'self'; worker-src blob:; img-src 'self' data: blob:; script-src 'self' 'nonce-\$request_id'; style-src 'self'" always;

    # Below added to allow rsso user header that has _ in it
    underscores_in_headers on;

    add_header X-XSS-Protection "1; mode=block";
    add_header X-Frame-Options "SAMEORIGIN";
    add_header X-Content-Type-Options nosniff;
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header 'Cache-Control' 'no-store';
    add_header 'Pragma' 'no-cache';

    set \$swagger_ui  http://hedge-swagger-ui:48048;
    set \$device_rest http://device-rest:59986;

    # Rewriting rules (variable usage required to avoid nginx crash if host not resolveable at time of boot)
    # resolver required to enable name resolution at runtime, points at docker DNS resolver

    location /hedge {
`cat "${corssnippet}"`

    # TODO: The resolver may not work in K8 environment
      resolver           127.0.0.11 valid=30s;
      proxy_pass         http://hedge-ui-server:48100;
      proxy_set_header   Host \$host;
      proxy_set_header   x-userId \$user_header;
      proxy_set_header   X-Consumer-Username \$user_header;
      proxy_set_header   Upgrade \$http_upgrade;
      proxy_set_header   Connection "Upgrade";
      proxy_set_header   X-CSRF-Token \$http_x_csrf_token;

      auth_request       /external/auth;
      auth_request_set   \$auth_status \$upstream_http_status;
      auth_request_set   \$user_resources \$upstream_http_x_user_resources;

      auth_basic            \$basic_auth_required;
      auth_basic_user_file /etc/nginx/auth/.htpasswd;

      add_header x-user-resources \$user_resources;
      add_header X-XSS-Protection "1; mode=block";
      add_header X-Frame-Options "SAMEORIGIN";
      add_header X-Content-Type-Options nosniff;
      add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
      add_header 'Cache-Control' 'no-store';
      add_header 'Pragma' 'no-cache';
      add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; form-action 'self'; worker-src blob:; img-src 'self' data:; script-src 'self' 'nonce-\$request_id'; style-src 'self'" always;

      js_header_filter   audit.log;
    }

    location ~* /hedge/api/v3/usr_mgmt/user/.+/resource_url/all {
      auth_basic         off;
      resolver           127.0.0.11 valid=30s;
      rewrite            /hedge/(.*) /\$1 break;
      proxy_pass         http://hedge-user-app-mgmt:48111;
      proxy_set_header   Host \$host;
      proxy_set_header   x-userId \$user_header;
      proxy_set_header   X-Consumer-Username \$user_header;

      js_header_filter   audit.log;
    }

    location /hedge/api/v3/usr_mgmt/user/auth_type {
      auth_basic         off;
      resolver           127.0.0.11 valid=30s;
      proxy_pass         http://hedge-ui-server:48100;
      proxy_set_header   Host \$host;
      proxy_set_header   x-userId \$user_header;
      proxy_set_header   X-Consumer-Username \$user_header;
      proxy_set_header   X-CSRF-Token \$http_x_csrf_token;

      add_header X-XSS-Protection "1; mode=block";
      add_header X-Frame-Options "SAMEORIGIN";
      add_header X-Content-Type-Options nosniff;
      add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
      add_header 'Cache-Control' 'no-store';
      add_header 'Pragma' 'no-cache';
      add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; form-action 'self'; worker-src blob:; img-src 'self' data:; script-src 'self' 'nonce-\$request_id'; style-src 'self'" always;

      js_content   auth.addCSRF;
    }

    location ~* /hedge/api/v3/usr_mgmt/user/auth_type_sub {
      auth_basic         off;
      resolver           127.0.0.11 valid=30s;
      rewrite            /hedge/(.*) /\$1 break;
      proxy_pass         http://hedge-user-app-mgmt:48111;
      proxy_set_header   Host \$host;
      proxy_set_header   x-userId \$user_header;
      proxy_set_header   X-Consumer-Username \$user_header;
      proxy_set_header   X-CSRF-Token \$http_x_csrf_token;
    }

    `echo "${RED_NODE_DEMO}"`

    # Added as separate location due to the Content-Security-Policy case
    location /hedge/hedge-node-red {
      auth_request       /external/auth;
      auth_request_set   \$auth_status \$upstream_http_status;
      auth_request_set   \$user_resources \$upstream_http_x_user_resources;
      auth_basic         \$basic_auth_required;
      auth_basic_user_file /etc/nginx/auth/.htpasswd;

      resolver           127.0.0.11 valid=30s;
      proxy_pass         http://hedge-ui-server:48100;
      proxy_set_header   Host \$host;
      proxy_set_header   x-userId \$user_header;
      proxy_set_header   X-Consumer-Username \$user_header;
      proxy_set_header   Upgrade \$http_upgrade;
      proxy_set_header   Connection "Upgrade";
      proxy_set_header   X-CSRF-Token \$http_x_csrf_token;

      add_header X-XSS-Protection "1; mode=block";
      add_header X-Frame-Options "SAMEORIGIN";
      add_header X-Content-Type-Options nosniff;
      add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
      add_header 'Cache-Control' 'no-store';
      add_header 'Pragma' 'no-cache';
      add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; form-action 'self'; worker-src blob:; img-src 'self' data:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'" always;

      js_header_filter   audit.log;
    }

    location /hedge/device-virtual {
        auth_request       /external/auth;
        auth_request_set   \$auth_status \$upstream_http_status;
        auth_request_set   \$user_resources \$upstream_http_x_user_resources;
        auth_basic         \$basic_auth_required;
        auth_basic_user_file /etc/nginx/auth/.htpasswd;

        resolver           127.0.0.11 valid=30s;
        rewrite            /hedge/device-virtual/(.*) /\$1 break;

        # Using variable in proxy_pass to avoid startup DNS resolution error when device-virtual is not running
        set \$device_virtual http://device-virtual:49991;
        proxy_pass         \$device_virtual;

        proxy_set_header   Host \$host;
        proxy_set_header   x-userId \$user_header;
        proxy_set_header   X-Consumer-Username \$user_header;
        proxy_set_header   Upgrade \$http_upgrade;
        proxy_set_header   Connection "Upgrade";
        proxy_set_header   X-CSRF-Token \$http_x_csrf_token;

        add_header X-XSS-Protection "1; mode=block";
        add_header X-Frame-Options "SAMEORIGIN";
        add_header X-Content-Type-Options nosniff;
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
        add_header 'Cache-Control' 'no-store';
        add_header 'Pragma' 'no-cache';
        add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; form-action 'self'; worker-src blob:; img-src 'self' data:; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'" always;

        js_header_filter   audit.log;
      }

    location /hedge/hedge-grafana {
        auth_request       /external/auth;
        auth_request_set   \$auth_status \$upstream_http_status;
        auth_request_set   \$user_resources \$upstream_http_x_user_resources;
        auth_basic         \$basic_auth_required;
        auth_basic_user_file /etc/nginx/auth/.htpasswd;

        resolver           127.0.0.11 valid=30s;
        proxy_pass         http://hedge-ui-server:48100;
        proxy_set_header   Host \$host;
        proxy_set_header   x-userId \$user_header;
        proxy_set_header   X-Consumer-Username \$user_header;
        proxy_set_header   X-CSRF-Token \$http_x_csrf_token;

        add_header X-XSS-Protection "1; mode=block";
        add_header X-Frame-Options "SAMEORIGIN";
        add_header X-Content-Type-Options nosniff;
        add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
        add_header 'Cache-Control' 'no-store';
        add_header 'Pragma' 'no-cache';

        js_header_filter   audit.log;
      }

    location / {
      auth_basic         off;
      resolver           127.0.0.11 valid=30s;
      proxy_pass         http://hedge-ui-server:48100;
      proxy_set_header   Host \$host;
      proxy_set_header   x-userId \$user_header;
      proxy_set_header   X-Consumer-Username \$user_header;
      proxy_set_header   X-CSRF-Token \$http_x_csrf_token;

      add_header X-XSS-Protection "1; mode=block";
      add_header X-Frame-Options "SAMEORIGIN";
      add_header X-Content-Type-Options nosniff;
      add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
      add_header 'Cache-Control' 'no-store';
      add_header 'Pragma' 'no-cache';
      add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; form-action 'self'; worker-src blob:; img-src 'self' data: blob:; script-src 'self' 'nonce-\$request_id'; style-src 'self' 'unsafe-inline'" always;

      js_header_filter   audit.log;
    }

    location /swagger {
      auth_basic         off;
      resolver           127.0.0.11 valid=30s;
      proxy_pass         http://hedge-swagger-ui:48048/;
      rewrite            ^/swagger$ /swagger/ permanent;

      proxy_set_header   Host \$host;
      proxy_set_header   x-userId \$user_header;
      proxy_set_header   X-Consumer-Username \$user_header;
      proxy_set_header   X-CSRF-Token \$http_x_csrf_token;

      add_header X-XSS-Protection "1; mode=block";
      add_header X-Frame-Options "SAMEORIGIN";
      add_header X-Content-Type-Options nosniff;
      add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
      add_header 'Cache-Control' 'no-store';
      add_header 'Pragma' 'no-cache';
      add_header Content-Security-Policy "default-src 'self'; base-uri 'self'; form-action 'self'; worker-src blob:; img-src 'self' data: blob:; script-src 'self' 'nonce-\$request_id'; style-src 'self' 'unsafe-inline'" always;

      add_header 'Access-Control-Allow-Origin' '*';
      add_header 'Access-Control-Allow-Methods' 'GET, POST, OPTIONS, PUT, DELETE';
      add_header 'Access-Control-Allow-Headers' 'Authorization, Content-Type';

      js_header_filter   audit.log;
    }

    include /etc/nginx/conf.d/generated-routes.inc;
    include /etc/nginx/conf.d/edgex-custom-rewrites.inc;

}

# Don't output NGINX version in Server: header
server_tokens off;

EOH

rm -f "${corssnippet}"

# Secure entrypoint script will block on opening a TCP listener after this script exits
