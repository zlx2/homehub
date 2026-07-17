FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /src/apps/control

COPY apps/control/go.mod ./go.mod
RUN go mod download

COPY apps/control/ .

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /out/homehub-control \
    ./cmd/homehub-control

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/homehub-control /homehub-control

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/homehub-control"]
CMD ["serve"]
