FROM node:24.17.0-alpine3.24 AS build

WORKDIR /app

ARG HTTP_PROXY=
ARG HTTPS_PROXY=
ARG http_proxy=
ARG https_proxy=

COPY services/hermes-terminal-web/package.json services/hermes-terminal-web/package-lock.json ./
RUN npm ci

COPY services/hermes-terminal-web/ ./
RUN npm run check && npm run build

FROM nginx:1.29.8-alpine

COPY services/hermes-terminal-web/nginx.conf /etc/nginx/nginx.conf
COPY --from=build /app/dist /usr/share/nginx/html/hermes

USER 101:101
EXPOSE 8080
