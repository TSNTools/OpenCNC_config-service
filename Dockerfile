# -------- Build Stage --------
FROM golang:1.24 AS build

ENV CONFIG_ROOT=/go/src/config-service
ENV CGO_ENABLED=0

RUN mkdir -p $CONFIG_ROOT

COPY . $CONFIG_ROOT

RUN cd $CONFIG_ROOT && GO111MODULE=on go build -o /go/bin/main ./


# -------- Runtime Stage --------
FROM alpine:3.11
# RUN apk add bash if needed
ENV HOME=/home/config-service
RUN mkdir -p $HOME
WORKDIR $HOME

COPY --from=build /go/bin/main /usr/local/bin/

EXPOSE 5150

CMD ["main"]
