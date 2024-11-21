# syntax=docker/dockerfile:1

FROM golang:1.22.7-alpine AS build

ARG COMMIT=not_generated

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X main.commitHash=$COMMIT" -o /api .

FROM scratch

COPY --from=build /api /api

EXPOSE 80

ENTRYPOINT ["/api"]
