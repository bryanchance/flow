FROM golang:alpine as build
RUN apk add -U make git
ARG VERSION
ARG BUILD
COPY . /src
WORKDIR /src
RUN make VERSION=$VERSION BUILD=$BUILD

FROM alpine:3.15
COPY --from=build /src/bin/fynca /usr/bin/fynca
ENTRYPOINT ["/usr/bin/fynca"]
CMD ["-h"]
