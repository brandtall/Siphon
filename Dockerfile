FROM golang:alpine AS builder

WORKDIR /app


COPY go.mod go.sum ./
RUN go mod download


COPY . .


ARG APP_DIR


RUN CGO_ENABLED=0 go build -o binary -v $APP_DIR


FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/binary .


RUN chmod +x binary

EXPOSE 5050/udp

CMD ["./binary"]
