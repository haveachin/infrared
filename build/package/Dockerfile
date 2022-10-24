FROM golang:1.18 AS builder
LABEL stage=intermediate
COPY . /infrared
WORKDIR /infrared
ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .

FROM alpine:latest
LABEL maintainer="Hendrik Jonas Schlehlein <hendrik.schlehlein@gmail.com>"
LABEL org.opencontainers.image.source https://github.com/haveachin/infrared
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY --from=builder /main ./
RUN chmod +x ./main
ENTRYPOINT [ "./main" ]