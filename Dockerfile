FROM golang:1.17.1-alpine3.14
RUN mkdir -p /go/src/github.com/hightoxicity/k8s-nodepool-cordon
WORKDIR /go/src/github.com/hightoxicity/k8s-nodepool-cordon
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s -v -extldflags -static" -a main.go


FROM docker:19.03.11 as static-docker-source

FROM alpine:3.14
COPY --from=0 /go/src/github.com/hightoxicity/k8s-nodepool-cordon/main /k8s-nodepool-cordon
ARG CLOUD_SDK_VERSION=358.0.0
ENV CLOUD_SDK_VERSION=$CLOUD_SDK_VERSION
ENV PATH /google-cloud-sdk/bin:$PATH
COPY --from=static-docker-source /usr/local/bin/docker /usr/local/bin/docker
RUN addgroup -g 1000 -S cloudsdk && \
    adduser -u 1000 -S cloudsdk -G cloudsdk
RUN apk --no-cache add \
        curl \
        python3 \
        py3-crcmod \
        py3-openssl \
        bash \
        libc6-compat \
        openssh-client \
        git \
        gnupg \
    && curl -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-${CLOUD_SDK_VERSION}-linux-x86_64.tar.gz && \
    tar xzf google-cloud-sdk-${CLOUD_SDK_VERSION}-linux-x86_64.tar.gz && \
    rm google-cloud-sdk-${CLOUD_SDK_VERSION}-linux-x86_64.tar.gz && \
    gcloud config set core/disable_usage_reporting true && \
    gcloud config set component_manager/disable_update_check true && \
    gcloud config set metrics/environment github_docker_image && \
    gcloud --version

#WORKDIR /
#RUN apk --no-cache add \
#        curl \
#        python3 \
#        py3-pip \
#        bash \
#        libc6-compat \
#        openssh-client \
#        git && \
#        pip3 install --upgrade pip && \
#        pip install wheel && \
#        pip install crcmod && \
#        curl -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-sdk-${GOOGLE_CLOUD_SDK_VERSION}-linux-x86_64.tar.gz && \
#        tar xzf google-cloud-sdk-${GOOGLE_CLOUD_SDK_VERSION}-linux-x86_64.tar.gz -C / && \
#        rm google-cloud-sdk-${GOOGLE_CLOUD_SDK_VERSION}-linux-x86_64.tar.gz && \
#        ln -s /lib /lib64 && \
#        gcloud config set core/disable_usage_reporting true && \
#        gcloud config set component_manager/disable_update_check true && \
#        gcloud config set metrics/environment github_docker_image && \
#        gcloud --version
ENTRYPOINT ["/k8s-nodepool-cordon"]
CMD ["-help"]
