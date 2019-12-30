FROM golang:latest AS builder
WORKDIR $GOPATH/src/github.com/haveachin/infrared
COPY . .
WORKDIR $GOPATH/src/github.com/haveachin/infrared/cmd/infrared
RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o /main .

FROM alpine:latest
LABEL maintainer="Hendrik Jonas Schlehlein <hendrik.schlehlein@gmail.com>"
RUN apk --no-cache add ca-certificates
COPY --from=builder /main ./
RUN mkdir configs
RUN chmod +x ./main
ENTRYPOINT [ "./main" ]
EXPOSE 25565