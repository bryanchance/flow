FROM golang:alpine as build
RUN apk add -U make git
COPY . /src
WORKDIR /src
RUN make

FROM alpine:3.15
COPY --from=build /src/bin/finca /usr/bin/finca
ENTRYPOINT ["/usr/bin/finca"]
CMD ["-h"]
