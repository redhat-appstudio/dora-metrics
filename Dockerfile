FROM golang:alpine
RUN apk add build-base
RUN mkdir /app
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY *.go ./
RUN go build -o exporters
EXPOSE 9101
CMD [ "./exporters" ]
