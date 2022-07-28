FROM golang:alpine as build
RUN apk add -U make git
ARG VERSION
ARG COMMIT
ARG BUILD
COPY . /src
WORKDIR /src
RUN make VERSION=$VERSION COMMIT=$COMMIT BUILD=$BUILD daemon

FROM alpine:3.16
COPY --from=build /src/bin/flow /usr/bin/flow
ENTRYPOINT ["/usr/bin/flow"]
CMD ["-h"]
