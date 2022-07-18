FROM golang:alpine as build
ARG WORKFLOW
ARG VERSION
ARG BUILD
RUN apk add -U make git
COPY . /src
WORKDIR /src
RUN make VERSION=$VERSION BUILD=$BUILD cmd/$WORKFLOW

FROM alpine:3.16
ARG WORKFLOW
COPY --from=build /src/bin/$WORKFLOW /usr/bin/flow-workflow
ENTRYPOINT ["/usr/bin/flow-workflow"]
CMD ["-h"]
