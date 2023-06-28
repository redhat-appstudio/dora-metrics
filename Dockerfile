FROM golang:alpine
RUN apk add build-base
RUN mkdir /app
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
RUN go get github.com/albarbaro/go-pagerduty@608750699b58fc256f313b33001030f994573169
COPY *.go ./
RUN go build -o exporters
EXPOSE 9101
CMD [ "./exporters" ]
