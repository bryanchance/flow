FROM golang:alpine as build
RUN apk add -U make git
ARG VERSION
ARG BUILD
COPY . /src
WORKDIR /src
RUN make VERSION=$VERSION BUILD=$BUILD daemon

FROM alpine:3.15
COPY --from=build /src/bin/flow /usr/bin/flow
ENTRYPOINT ["/usr/bin/flow"]
CMD ["-h"]
