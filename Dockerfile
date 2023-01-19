FROM node:18-bullseye

WORKDIR /workspace

COPY audon-fe/ /workspace/

RUN npm install && \
    npm run build

FROM golang:1.19-bullseye

WORKDIR /workspace

COPY go.mod /workspace/go.mod
COPY go.sum /workspace/go.sum
RUN go mod download -x

COPY *.go /workspace/

RUN go build -v -o audon-bin .

FROM debian:bullseye

WORKDIR /audon

COPY --from=0 /workspace/dist /audon/audon-fe/dist
COPY --from=1 /workspace/audon-bin /audon/
COPY locales /audon/locales

RUN echo "Etc/UTC" > /etc/localtime && \
    apt-get update && apt-get upgrade -y && \
    apt-get -y --no-install-recommends install \
    tini \
    tzdata \
    ca-certificates

ENV AUDON_ENV=production

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["/audon/audon-bin"]

EXPOSE 8100
