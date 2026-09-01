# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
# Vite bakes VITE_* variables into the JS bundle at build time — there is
# no runtime env var to change this afterward. Default to "" (same-origin
# relative requests), which is correct behind the Ingress that puts web
# and api under one host in deploy/k8s; local dev overrides it to
# http://localhost:8080 via the repo-root .env instead of this build arg.
ARG VITE_API_URL=""
ENV VITE_API_URL=${VITE_API_URL}
RUN npm run build

# nginx-unprivileged: same nginx, but listens on a port >1024 and runs as
# a non-root user by default, so no extra hardening is needed here.
FROM nginxinc/nginx-unprivileged:1.27-alpine AS final
COPY --from=build /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 8080
