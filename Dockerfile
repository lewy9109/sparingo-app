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
USER app
ENV POSTGRES_MIGRATIONS_DIR=/app/migrations/postgres
EXPOSE 8080
ENTRYPOINT ["/app/sqoush"]
