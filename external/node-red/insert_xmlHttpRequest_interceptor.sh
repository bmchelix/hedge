#!/bin/sh
# Prepend the interceptor script to red.min.js
INTERCEPTOR_SCRIPT="/tmp/xmlHttpRequest_interceptor.js"
TARGET_FILE="/usr/src/node-red/node_modules/@node-red/editor-client/public/red/red.min.js"

 # Check if the interceptor script and target file exist
if [ -f "$INTERCEPTOR_SCRIPT" ] && [ -f "$TARGET_FILE" ]; then
    # Prepend the JavaScript code from xmlHttpRequest_interceptor.js to red.min.js
    cat $INTERCEPTOR_SCRIPT $TARGET_FILE > $TARGET_FILE.new
    mv $TARGET_FILE.new $TARGET_FILE
else
    echo "Required files are missing."
fi
