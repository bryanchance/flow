FROM golang:1.18 as build
ARG PROCESSOR
ARG COMMIT
ARG VERSION
ARG BUILD
COPY . /src
WORKDIR /src
RUN make VERSION=$VERSION COMMIT=$COMMIT BUILD=$BUILD cmd/$PROCESSOR

FROM ubuntu:22.04
ARG PROCESSOR
RUN apt-get update && apt-get install -y blender
COPY --from=build /src/bin/$PROCESSOR /usr/bin/flow-processor
ENTRYPOINT ["/usr/bin/flow-processor"]
CMD ["-h"]
