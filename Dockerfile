FROM node:24-alpine AS web
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build:web

FROM golang:1.27-alpine AS build
WORKDIR /src
ARG TRENDINARY_RELEASE=dev
ARG TRENDINARY_COMMIT_SHA=unknown

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/web/dist ./internal/web/dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X github.com/chrisbirster/trendinary/internal/buildinfo.Release=${TRENDINARY_RELEASE} -X github.com/chrisbirster/trendinary/internal/buildinfo.Commit=${TRENDINARY_COMMIT_SHA}" \
    -o /out/trendinary ./cmd/trendinary

FROM alpine:3.22
RUN apk add --no-cache ca-certificates \
    && addgroup -S trendinary \
    && adduser -S -G trendinary trendinary

COPY --from=build /out/trendinary /usr/local/bin/trendinary

USER trendinary
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/trendinary"]
