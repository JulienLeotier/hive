FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/translator .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/translator /usr/local/bin/translator
USER nonroot:nonroot
EXPOSE 9102
ENTRYPOINT ["/usr/local/bin/translator"]
