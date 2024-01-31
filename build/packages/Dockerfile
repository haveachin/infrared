FROM --platform=$BUILDPLATFORM golang:alpine AS builder
ARG TARGETARCH
WORKDIR /tmp/build
COPY . /tmp/build
ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags "-s -w" -o ./out/infrared ./cmd/infrared
RUN chmod +x ./out/infrared

FROM alpine:latest
COPY --from=builder /tmp/build/out/infrared /usr/bin/
WORKDIR /etc/infrared
ENTRYPOINT [ "/usr/bin/infrared" ]