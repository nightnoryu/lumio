FROM alpine:3.23.3

RUN apk add --no-cache ca-certificates curl && \
    update-ca-certificates && \
    addgroup -g 1001 lumio-user && \
    adduser -u 1001 -D -G lumio-user -s /sbin/nologin -g "go service user" lumio-user

COPY ./bin/lumio /app/bin/
WORKDIR /app

USER lumio-user

EXPOSE 8080
ENTRYPOINT [ "/app/bin/lumio" ]
