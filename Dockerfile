FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/rss2telegram .

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /app
COPY --from=build /out/rss2telegram /app/rss2telegram
VOLUME ["/data"]
ENTRYPOINT ["/app/rss2telegram"]
CMD ["-config", "/data/config.yaml"]
