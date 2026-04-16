FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o awm-cli ./cmd/awm-cli

FROM alpine:3.22
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/awm-cli /awm-cli
ENTRYPOINT ["/awm-cli"]