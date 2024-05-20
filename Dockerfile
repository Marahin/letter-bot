FROM golang:1.22-alpine AS build
RUN apk update && apk add ca-certificates && apk add tzdata && apk add git && apk add make
WORKDIR /build
COPY . .
RUN git config --global --add safe.directory /build
RUN make install-dependencies && CGO_ENABLED=0 GOOS=linux make build-only

FROM scratch AS final
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /build/bin/spot-assistant-bot spot-assistant-bot
ENV TZ Europe/Berlin
ENTRYPOINT ["/spot-assistant-bot"]
