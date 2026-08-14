FROM golang:1.26-alpine3.23 AS builder
WORKDIR /home/src
COPY . .
RUN GOPROXY='https://proxy.golang.org,direct' CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -o=./store-server ./cmd/web
RUN chmod +x /home/src/entrypoint/entrypoint.sh

FROM debian:bookworm-slim
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /home/src/store-server /bin/store-server
COPY --from=builder /home/src/entrypoint/entrypoint.sh /bin/entrypoint.sh
ENTRYPOINT ["/bin/entrypoint.sh"]
EXPOSE 8080 4445
