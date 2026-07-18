FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /src
COPY packages/go-sdk ./packages/go-sdk
COPY services/drop ./services/drop
WORKDIR /src/services/drop
RUN go test ./... && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/homehub-drop ./cmd/homehub-drop

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/homehub-drop /homehub-drop

USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/homehub-drop"]
CMD ["serve"]
