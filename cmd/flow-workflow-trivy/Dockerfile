FROM golang:alpine as build
ARG PROCESSOR
ARG VERSION
ARG BUILD
RUN apk add -U make git
COPY . /src
WORKDIR /src
RUN make VERSION=$VERSION BUILD=$BUILD cmd/$PROCESSOR

FROM golang:alpine as trivy
RUN apk add -U make git build-base gcc
WORKDIR /src
RUN git clone --depth 1 --branch v0.29.2 https://github.com/aquasecurity/trivy && \
	cd trivy/cmd/trivy/ && \
	go build -v .

FROM alpine:3.16
ARG PROCESSOR
COPY --from=build /src/bin/$PROCESSOR /usr/bin/flow-processor
COPY --from=trivy /src/trivy/cmd/trivy/trivy /usr/bin/trivy
ENTRYPOINT ["/usr/bin/flow-processor"]
CMD ["-h"]
