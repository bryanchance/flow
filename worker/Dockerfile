FROM ubuntu:20.04 as build
RUN apt-get update && apt-get install -y ca-certificates curl xz-utils
RUN curl -sSL https://mirror.clarkson.edu/blender/release/Blender2.92/blender-2.92.0-linux64.tar.xz -o /tmp/blender.tar.xz && \
	cd /opt && tar xvf /tmp/blender.tar.xz && \
	mv blender-2.92.0-linux64 blender

FROM golang:1.16 as compositor
COPY . /app
WORKDIR /app
RUN make finca-compositor

FROM ubuntu:20.04
RUN apt-get update && apt-get install -y curl ca-certificates gnupg2
RUN distribution=$(. /etc/os-release;echo $ID$VERSION_ID) \
    && curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | apt-key add - \
    && curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | tee /etc/apt/sources.list.d/nvidia-docker.list
RUN apt-get update && apt-get install -y nvidia-container-runtime nvidia-container-toolkit libx11-6 libxi6 libxxf86vm1 libxfixes3 libxrender1 libgl1 libglu1-mesa ffmpeg imagemagick
RUN apt-get install -y unzip jq
COPY --from=build /opt/blender /opt/blender
RUN curl -sSL https://dl.min.io/client/mc/release/linux-amd64/mc -o /usr/local/bin/mc && \
    chmod +x /usr/local/bin/mc
RUN curl -sSL https://releases.hashicorp.com/nomad/1.0.5/nomad_1.0.5_linux_amd64.zip -o /tmp/nomad.zip && \
    unzip -d /usr/local/bin/ /tmp/nomad.zip && \
    rm -rf /tmp/nomad.zip
COPY worker/render.sh /usr/local/bin/render.sh
COPY --from=compositor /app/bin/finca-compositor /usr/local/bin/finca-compositor
RUN echo "PATH=/opt/blender:$PATH" >> /etc/profile
CMD ["/usr/local/bin/render.sh"]
