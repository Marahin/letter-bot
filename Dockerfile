# Debian-based (not Alpine): the Tailwind standalone CLI is glibc-linked.
FROM golang:1.27-bookworm AS build
# The base image pins GOTOOLCHAIN=local and can lag go.mod's patch version.
ENV GOTOOLCHAIN=auto
ARG TAILWIND_VERSION=v3.4.17
ARG TAG=
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata git make curl \
    && rm -rf /var/lib/apt/lists/*
RUN curl -fsSL -o /usr/local/bin/tailwindcss \
        https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWIND_VERSION}/tailwindcss-linux-x64 \
    && chmod +x /usr/local/bin/tailwindcss
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN git config --global --add safe.directory /build
RUN make css TAILWIND=/usr/local/bin/tailwindcss \
    && CGO_ENABLED=0 GOOS=linux make build-only

FROM scratch AS web
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /build/bin/letter-web /letter-web
ENV TZ=Europe/Berlin
EXPOSE 8080 3005
ENTRYPOINT ["/letter-web"]

# Keep bot last: `docker build .` must keep producing the bot image.
FROM scratch AS bot
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /build/bin/spot-assistant-bot /spot-assistant-bot
ENV TZ=Europe/Berlin
ENTRYPOINT ["/spot-assistant-bot"]
