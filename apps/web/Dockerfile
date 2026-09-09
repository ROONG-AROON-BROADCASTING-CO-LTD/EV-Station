FROM node:22-alpine AS build
RUN corepack enable
WORKDIR /src
COPY package.json ./
COPY pnpm-workspace.yaml ./
RUN pnpm install --no-frozen-lockfile
COPY . .
ARG VITE_API_BASE_URL=/api/v1
ARG VITE_GOOGLE_MAPS_BROWSER_API_KEY=
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_GOOGLE_MAPS_BROWSER_API_KEY=$VITE_GOOGLE_MAPS_BROWSER_API_KEY
RUN pnpm build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
