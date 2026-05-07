FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN go build -o /out/pullsing-server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/pullsing-server /usr/local/bin/pullsing-server

EXPOSE 8080

ENTRYPOINT ["pullsing-server"]
