FROM golang:alpine as build
ARG PROCESSOR
ARG VERSION
ARG COMMIT
ARG BUILD
RUN apk add -U make git
COPY . /src
WORKDIR /src
RUN make VERSION=$VERSION COMMIT=$COMMIT BUILD=$BUILD cmd/$PROCESSOR

FROM alpine:3.16
RUN apk add -U ffmpeg
ARG PROCESSOR
COPY --from=build /src/bin/$PROCESSOR /usr/bin/flow-processor
ENTRYPOINT ["/usr/bin/flow-processor"]
CMD ["-h"]
