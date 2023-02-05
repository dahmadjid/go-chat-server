FROM golang:1.19

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY server/main.go main.go

RUN go build

CMD ["./server.exe"]