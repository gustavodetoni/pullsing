FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY proto ./proto

RUN go build -o /out/pullsing-server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=build /out/pullsing-server /app/pullsing-server
COPY migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["./pullsing-server"]
