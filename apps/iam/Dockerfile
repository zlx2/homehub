FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /src
COPY packages/go-sdk /src/packages/go-sdk
COPY apps/iam /src/apps/iam
WORKDIR /src/apps/iam

ARG VERSION=dev
ARG HTTP_PROXY=
ARG HTTPS_PROXY=
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/homehub-iam ./cmd/homehub-iam

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/homehub-iam /homehub-iam
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/homehub-iam"]
