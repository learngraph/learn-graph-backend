FROM golang:1.18 AS builder
COPY . /build
WORKDIR /build
RUN make

FROM alpine
COPY --from=builder /build/main /app
CMD ["/app"]
