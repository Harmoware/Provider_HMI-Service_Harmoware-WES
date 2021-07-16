FROM golang:latest


WORKDIR /go/src/

RUN apt-get update
RUN apt-get -y install zip unzip

ADD . /go/src

COPY . .

# build
RUN go build
ENTRYPOINT ./hmi-service

# expose port
EXPOSE 10090

# ADD . /go/src/github.com/Harmoware/Provider_HMI-Service_Harmoware-WES

# RUN go install github.com/Harmoware/Provider_HMI-Service_Harmoware-WES@latest

# ENTRYPOINT /go/bin/hmi-service

# EXPOSE 10090