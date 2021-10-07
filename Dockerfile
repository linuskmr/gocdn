# Dockerfile inspired by the one at https://hub.docker.com/_/golang
# Build: docker build -t gocdn .
# Run root server: docker run -p 8192:8192 gocdn root_server
# Run cdn server: docker run -p 8193:8193 gocdn cdn_server

FROM golang:1.17

WORKDIR /go/src/app
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...