FROM golang:1.23-bullseye as build

ADD . /src/friendbot
WORKDIR /src/friendbot
RUN go build -o /bin/friendbot ./services/friendbot

FROM ubuntu:22.04

RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends ca-certificates
COPY --from=build /bin/friendbot /app/
EXPOSE 8004
ENTRYPOINT ["/app/friendbot"]
