FROM node:24.17.0-alpine3.24 AS build

WORKDIR /app

ARG HTTP_PROXY=
ARG HTTPS_PROXY=
ARG http_proxy=
ARG https_proxy=

COPY apps/portal/package.json apps/portal/package-lock.json ./
RUN npm ci

COPY apps/portal/ ./
RUN npm run check && npm run build

FROM nginx:1.29.8-alpine

COPY apps/portal/nginx.conf /etc/nginx/nginx.conf
COPY --from=build /app/dist /usr/share/nginx/html

USER 101:101
EXPOSE 8080
