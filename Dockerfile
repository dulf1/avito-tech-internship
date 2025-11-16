FROM golang:1.23 AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o prservice ./cmd/app

FROM alpine:3.19

WORKDIR /app

COPY --from=build /app/prservice /usr/local/bin/prservice
COPY migrations ./migrations

ENV HTTP_ADDR=":8080"

CMD ["prservice"]
