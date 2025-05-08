FROM aus-harboreg-01.bmc.com/psg_base_images/chainguard:latest-wolfi.base

ARG REDIS_VER

RUN apk update && apk add --no-cache \
    libgcc \
    bash \
    curl \
    redis-cli \
    redis=${REDIS_VER}

WORKDIR /data

# Create a group 'edgex' with GID 2001 and a user 'edgex' with UID 2002
RUN addgroup -S -g 2001 edgex && adduser -S -u 2002 -G edgex edgex

RUN mkdir /usr/local/bin

COPY --chown=edgex:edgex external/redis/redisearch.so /usr/local/bin/redisearch.so

COPY --chown=edgex:edgex external/redis/docker-entrypoint.sh /usr/local/bin

RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Switch to non-root user
USER 2002

EXPOSE 6379