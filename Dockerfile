FROM golang:1.25-bookworm AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /app/sqoush ./main.go

FROM alpine:3.19
RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /app/sqoush /app/sqoush
COPY --from=build /app/templates /app/templates
COPY --from=build /app/static /app/static
COPY --from=build /app/migrations /app/migrations
RUN mkdir -p /data && chown app:app /data
USER app
ENV DB_PATH=/data/sqoush.db
ENV DB_MIGRATIONS_DIR=/app/migrations
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/sqoush"]
