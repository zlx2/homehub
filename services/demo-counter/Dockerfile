FROM rust:1.97.0-alpine3.24 AS build

ENV CARGO_REGISTRIES_CRATES_IO_PROTOCOL=sparse \
    CARGO_HTTP_MULTIPLEXING=false

RUN sed -i 's@dl-cdn.alpinelinux.org@mirrors.tuna.tsinghua.edu.cn@g' /etc/apk/repositories \
    && unset HTTP_PROXY HTTPS_PROXY http_proxy https_proxy \
    && apk add --no-cache musl-dev
WORKDIR /src
COPY services/demo-counter/Cargo.toml services/demo-counter/Cargo.lock ./
COPY services/demo-counter/src ./src

RUN cargo build --locked --release && strip target/release/demo-counter

FROM build AS test

RUN cargo test --locked

FROM alpine:3.24

COPY --from=build /src/target/release/demo-counter /demo-counter

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/demo-counter"]
CMD ["serve"]
