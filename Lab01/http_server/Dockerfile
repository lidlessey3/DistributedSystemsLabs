FROM golang:1.21 as builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY *.go ./

RUN GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o ./http_server

RUN mkdir public

COPY public/* public/

ENTRYPOINT ["./http_server"]
