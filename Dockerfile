FROM golang:latest AS builder
LABEL stage=intermediate
COPY . /infrared
WORKDIR /infrared
ENV GO111MODULE=on
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .

FROM alpine:latest
LABEL maintainer="Hendrik Jonas Schlehlein <hendrik.schlehlein@gmail.com>"
RUN apk --no-cache add ca-certificates
WORKDIR /
RUN mkdir configs
COPY --from=builder /main ./
RUN chmod +x ./main
ENTRYPOINT [ "./main" ]
