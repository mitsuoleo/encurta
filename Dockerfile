FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -H -u 65532 app
WORKDIR /app
COPY --from=build /out/api /app/api
COPY migrations /migrations
RUN chown -R app:app /app /migrations
ENV MIGRATIONS_PATH=file:///migrations
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/api"]
