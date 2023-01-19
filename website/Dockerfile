FROM alpine:3
RUN apk add --no-cache curl
WORKDIR /website
COPY . .
RUN curl -sL https://github.com/tailwindlabs/tailwindcss/releases/download/v3.2.4/tailwindcss-linux-x64 -o tailwindcss && \
	chmod +x tailwindcss
RUN ./tailwindcss -i ./src/styles.css -o ./src/styles.min.css --minify
