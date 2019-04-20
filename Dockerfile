FROM golang:alpine as builder
LABEL maintainer="Ross Smith II <ross@smithii.com>"

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH /go

COPY . /go/src/github.com/rasa/feedster

RUN set -x \
	&& apk add --no-cache --virtual .build-deps \
		git \
		gcc \
		libc-dev \
		libgcc \
		make \
	&& cd /go/src/github.com/rasa/feedster \
	&& make vendor static \
	&& mv feedster /usr/bin/feedster \
	&& apk del .build-deps \
	&& rm -rf /go \
	&& echo "Build complete."

FROM alpine:latest

COPY --from=builder /usr/bin/feedster /usr/bin/feedster

ENTRYPOINT [ "feedster" ]
CMD [ "--help" ]
