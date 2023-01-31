FROM golang:1.19 AS builder
LABEL stage=intermediate
ARG VERSION
COPY . /infrared
WORKDIR /infrared
ENV GO111MODULE=on
RUN make swag
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=$VERSION" -o /main .

FROM alpine:latest
LABEL maintainer="Hendrik Jonas Schlehlein <hendrik.schlehlein@gmail.com>"
LABEL org.opencontainers.image.source https://github.com/haveachin/infrared
ENV INFRARED_WORKING_DIR=/infrared
RUN apk --no-cache add ca-certificates
WORKDIR /
COPY --from=builder /main ./
RUN chmod +x ./main
ENTRYPOINT [ "./main" ]