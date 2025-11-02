FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25 AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ENV VERSION=$VERSION

WORKDIR /app/
ADD . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
    -ldflags="-X main.version=$VERSION" \
    -o github-stars \
    .

FROM alpine

RUN apk update && \
    apk add --no-cache tzdata

WORKDIR /app
COPY --from=builder /app/github-stars /app/github-stars

RUN /usr/sbin/addgroup app
RUN /usr/sbin/adduser app -G app -D
USER app

ENTRYPOINT ["/app/github-stars"]
CMD []
