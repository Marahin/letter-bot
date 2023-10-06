FROM golang:1.21.1 AS build
WORKDIR /build
ENV CGO_ENABLED=0
COPY . .
RUN go build -o spot-assistant-bot -buildvcs=false cmd/main.go

FROM alpine
WORKDIR /app
RUN apk add --no-cache tzdata
ENV CGO_ENABLED=0
COPY --from=build /build/spot-assistant-bot spot-assistant-bot
CMD ["/app/spot-assistant-bot"]
