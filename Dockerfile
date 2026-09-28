# syntax=docker/dockerfile:1
FROM node:26.8.1-alpine3.23 AS frontend
WORKDIR /src/web
RUN npm install --global pnpm@12.3.1
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
COPY api/ /src/api/
RUN pnpm run generate && pnpm run build

FROM golang:1.26.8-alpine3.23 AS build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY api/ /src/api/
RUN go run github.com/ogen-go/ogen/cmd/ogen@v1.24.0 --clean --target ./api/server/publicapi --package publicapi --config ./api/server/publicapi/.ogen.yml ../api/publicapi.yml \
    && go run github.com/a-h/templ/cmd/templ@v0.3.1020 generate
COPY --from=frontend /src/backend/internal/webui/dist/ ./internal/webui/dist/
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -o /out/lumio ./cmd/lumio

FROM alpine:3.23.3 AS runtime
RUN apk add --no-cache ca-certificates && addgroup -g 1001 lumio && adduser -u 1001 -D -G lumio lumio
WORKDIR /app
COPY --from=build /out/lumio /app/bin/lumio
USER 1001:1001
ENTRYPOINT ["/app/bin/lumio"]

FROM runtime AS worker
USER root
RUN apk add --no-cache vips-tools font-dejavu
USER 1001:1001
ENV VIPS_CONCURRENCY=2
CMD ["worker"]

FROM runtime AS web
EXPOSE 8080
