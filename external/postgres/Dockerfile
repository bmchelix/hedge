ARG POSTGRES_VER
FROM postgres:${POSTGRES_VER}

# Install su-exec and remove gosu
RUN apk add --no-cache su-exec && \
    rm -f /usr/local/bin/gosu

# Replace gosu with su-exec using a symlink
RUN ln -s /sbin/su-exec /usr/local/bin/gosu

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -g 2001 -S edgex && adduser -u 2002 -S -G edgex edgex

# Switch to non-root user
USER 2002