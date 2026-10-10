FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
ARG NPM_REGISTRY=https://registry.npmjs.org
RUN --mount=type=cache,target=/root/.npm npm ci --legacy-peer-deps --no-audit --no-fund --registry="$NPM_REGISTRY" --fetch-retries=4
COPY . .
RUN GENERATE_SOURCEMAP=false CI=false npm run build

FROM nginx:stable-alpine
ENV WATCHALERT_BACKEND_HOST=w8t-service WATCHALERT_BACKEND_PORT=9001
COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY --from=build /app/build /usr/share/nginx/html
EXPOSE 80
