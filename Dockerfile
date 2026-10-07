FROM golang:1.15-alpine as builder

RUN apk add --no-cache git ca-certificates

RUN go env -w GO111MODULE=on && \
    go env -w GOPROXY=https://goproxy.cn,direct

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

# Build this checkout, including its committed production frontend assets.
COPY . .
RUN CGO_ENABLED=0 go build -o /app/bin/goscheduler ./cmd/goscheduler

FROM alpine:3.12

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -g app app

RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime

WORKDIR /app

COPY --from=builder /app/bin/goscheduler .
COPY LICENSE .

RUN chown -R app:app ./

EXPOSE 5920

USER app

ENTRYPOINT ["/app/goscheduler", "web"]
