# Minimal wrapper that adds /bin/sh to the stock OpenFGA image
# so the entrypoint script can read the database password file.
FROM alpine:3.22 AS shell
FROM openfga/openfga:v1.15.1
COPY --from=shell /bin/sh /bin/sh
COPY --from=shell /lib/ld-musl-*.so* /lib/
COPY --chmod=755 openfga-entrypoint.sh /openfga-entrypoint.sh
ENTRYPOINT ["/bin/sh", "/openfga-entrypoint.sh"]
