FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /src
COPY apps/iam/go.mod ./
COPY apps/iam/ ./

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/homehub-iam ./cmd/homehub-iam

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/homehub-iam /homehub-iam
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/homehub-iam"]
