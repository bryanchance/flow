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
RUN if [ "$(uname -m)" != "x86_64" ]; then echo "ERR: only x86_64 is supported for containerized blender workflows"; exit 1; fi
RUN apt-get update && apt-get install -y curl ca-certificates gnupg2 software-properties-common
RUN distribution=$(. /etc/os-release;echo $ID$VERSION_ID) \
    && curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | apt-key add - \
    && curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | tee /etc/apt/sources.list.d/nvidia-docker.list
RUN apt-get update && apt-get install -y \
	nvidia-container-runtime \
	nvidia-container-toolkit \
	libx11-6 \
	libxi6 \
	libxxf86vm1 \
	libxfixes3 \
	libxrender1 \
	libgl1 \
	libglu1-mesa \
	imagemagick
ENV RELEASE=3.2
ENV VERSION=3.2.0
RUN curl -sSL https://mirrors.ocf.berkeley.edu/blender/release/Blender$RELEASE/blender-$VERSION-linux-x64.tar.xz -o /tmp/blender.tar.xz && \
	tar xvf /tmp/blender.tar.xz -C /opt && \
	mv /opt/blender* /opt/blender
ENV PATH="/opt/blender:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
COPY --from=build /src/bin/$PROCESSOR /usr/bin/flow-processor
ENTRYPOINT ["/usr/bin/flow-processor"]
CMD ["-h"]
