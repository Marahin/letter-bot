FROM golang:1.21.3-alpine AS build
RUN apk update && apk add ca-certificates && apk add tzdata && apk add git && apk add make
WORKDIR /build
COPY . .
RUN git config --global --add safe.directory /build
RUN CGO_ENABLED=0 GOOS=linux make build

FROM scratch AS final
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /build/spot-assistant-bot spot-assistant-bot
ENV TZ Europe/Berlin
ENTRYPOINT ["/spot-assistant-bot"]
