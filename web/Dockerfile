# syntax=docker/dockerfile:1

FROM node:24-alpine AS build
WORKDIR /web

RUN --mount=type=bind,source=package.json,target=package.json \
    --mount=type=bind,source=package-lock.json,target=package-lock.json \
    --mount=type=cache,target=/root/.npm \
    npm ci

COPY . .
RUN npm run build

FROM nginxinc/nginx-unprivileged:1.27-alpine AS runtime

COPY --chmod=0644 --link nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build --link /web/dist /usr/share/nginx/html

EXPOSE 8080

LABEL org.opencontainers.image.source="https://github.com/arasemco/stacksmith"

