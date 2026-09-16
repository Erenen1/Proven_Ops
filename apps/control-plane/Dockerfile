FROM golang:1.23-alpine AS builder

WORKDIR /workspace

COPY proto/ ./proto/
COPY apps/control-plane/ ./apps/control-plane/

WORKDIR /workspace/apps/control-plane
ENV GOWORK=off
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/control-plane ./cmd/server

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /bin/control-plane /app/control-plane

EXPOSE 8080 9090

ENTRYPOINT ["/app/control-plane"]
