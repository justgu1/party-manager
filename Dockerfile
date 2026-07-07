# syntax=docker/dockerfile:1

# ---- Stage 1: build the Svelte SPA ----------------------------------------
FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install
COPY web/ ./
RUN npm run build

# ---- Stage 2: build the Go binaries ---------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Use the freshly built SPA (overwrites the placeholder dist committed in repo).
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

# ---- Stage 3a: the API runtime (tiny) -------------------------------------
FROM alpine:3.20 AS api
RUN apk add --no-cache ca-certificates
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]

# ---- Stage 3b: the worker runtime (needs Chromium for chromedp) -----------
FROM alpine:3.20 AS worker
RUN apk add --no-cache ca-certificates chromium nss freetype harfbuzz ttf-freefont
ENV CHROME_BIN=/usr/bin/chromium-browser
COPY --from=build /out/worker /usr/local/bin/worker
ENTRYPOINT ["/usr/local/bin/worker"]
