FROM alpine:3.12

WORKDIR /opt/varys
RUN apk add -U tzdata ca-certificates

ARG TARGETOS
ARG TARGETARCH
COPY dist/varys_${TARGETOS}_${TARGETARCH}/varys /opt/varys/bin/varys
COPY LICENSE /opt/varys/LICENSE

ENTRYPOINT [ "/opt/varys/bin/varys" ]
