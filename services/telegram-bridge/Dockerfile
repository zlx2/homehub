FROM golang:1.26.5-alpine3.24 AS build
WORKDIR /src/services/telegram-bridge
COPY services/telegram-bridge/go.mod ./
COPY services/telegram-bridge/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telegram-bridge ./cmd/telegram-bridge

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/telegram-bridge /telegram-bridge
USER 65532:65532
EXPOSE 8730
ENTRYPOINT ["/telegram-bridge"]
