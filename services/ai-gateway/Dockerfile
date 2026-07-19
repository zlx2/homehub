FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /src/services/ai-gateway
COPY packages/go-sdk /src/packages/go-sdk
COPY services/ai-gateway/go.mod ./
COPY services/ai-gateway/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/ai-gateway ./cmd/ai-gateway

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/ai-gateway /ai-gateway
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/ai-gateway"]
