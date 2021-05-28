FROM golang:alpine as build
RUN apk add -U make git
COPY . /src
WORKDIR /src
RUN make

FROM r.underland.io/alpine:3.13
COPY --from=build /src/bin/finca /usr/bin/finca
CMD ["/usr/bin/finca"]
