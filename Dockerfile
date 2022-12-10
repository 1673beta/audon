FROM node:18-bullseye AS build

WORKDIR /workspace

COPY audon-fe/ /workspace/

RUN npm install && npm run build

FROM golang:1.19-bullseye

WORKDIR /audon

COPY --from=build /workspace/dist /audon/audon-fe/dist
COPY go.mod /audon/go.mod
COPY go.sum /audon/go.sum

RUN go mod download

COPY *.go /audon/

RUN go build -a -v -o audon-bin .

ENV AUDON_ENV=production

ENTRYPOINT ["/audon/audon-bin"]
EXPOSE 8100
