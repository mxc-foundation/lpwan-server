FROM golang:1.12-alpine AS development

ENV PROJECT_PATH=/network-server
ENV PATH=$PATH:$PROJECT_PATH/build
ENV CGO_ENABLED=0
ENV GO_EXTRA_BUILD_ARGS="-a -installsuffix cgo"

RUN apk add --no-cache ca-certificates tzdata make git bash protobuf

RUN mkdir -p $PROJECT_PATH
COPY . $PROJECT_PATH
WORKDIR $PROJECT_PATH

RUN make dev-requirements
RUN make clean build

FROM alpine:latest AS production

RUN mkdir -p $PROJECT_PATH
WORKDIR $PROJECT_PATH
RUN apk --no-cache add ca-certificates tzdata
COPY --from=development /loraserver/build/ $PROJECT_PATH
COPY --from=development /loraserver/configuration/ $PROJECT_PATH
ENTRYPOINT ["./loraserver"]
