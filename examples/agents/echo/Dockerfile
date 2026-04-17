FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/echo .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/echo /usr/local/bin/echo
USER nonroot:nonroot
EXPOSE 9100
ENTRYPOINT ["/usr/local/bin/echo"]
