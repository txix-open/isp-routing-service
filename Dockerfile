FROM golang:1.24-alpine as builder
WORKDIR /build
ARG version
ENV version_env=$version
ARG app_name
ENV app_name_env=$app_name
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    go build -ldflags="-X 'main.version=$version_env'" -o /main .

FROM alpine:3.21

RUN apk add --no-cache tzdata
RUN cp /usr/share/zoneinfo/Europe/Moscow /etc/localtime
RUN echo "Europe/Moscow" > /etc/timezone

ARG UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    appuser

RUN mkdir -p /app/data
RUN chown  appuser:appuser /app/data
VOLUME /app/data

USER appuser

WORKDIR /app

ARG app_name
ENV app_name_env=$app_name
COPY --from=builder main /app/$app_name_env
COPY /conf/config.yml /app/config.yml
COPY /conf/default_remote_config.json* /app/default_remote_config.json

ENTRYPOINT /app/$app_name_env
