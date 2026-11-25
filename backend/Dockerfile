FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/crm-tasks ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
ENV TZ=Europe/Moscow
COPY --from=build /out/crm-tasks /usr/local/bin/crm-tasks
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/crm-tasks"]
