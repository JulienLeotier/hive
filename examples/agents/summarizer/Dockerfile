FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod ./
COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/summarizer .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /out/summarizer /usr/local/bin/summarizer
USER nonroot:nonroot
EXPOSE 9101
ENTRYPOINT ["/usr/local/bin/summarizer"]
