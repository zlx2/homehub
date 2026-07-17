FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /src
COPY services/demo-decider/go.mod ./
COPY services/demo-decider/ .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/demo-decider .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/demo-decider /demo-decider

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/demo-decider"]
CMD ["serve"]
