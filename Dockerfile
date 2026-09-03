# Build Stage
FROM golang:1.24-alpine3.22 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/read-waka-stats ./cmd/read-waka-stats

# Runtime Stage
FROM alpine:3.21

RUN apk --no-cache add ca-certificates git tzdata

WORKDIR /github/workspace

COPY --from=builder /out/read-waka-stats /usr/local/bin/read-waka-stats

ENTRYPOINT ["read-waka-stats"]
