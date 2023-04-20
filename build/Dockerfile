FROM debian:buster-slim

RUN apt update -y && apt install ca-certificates -y

WORKDIR /app

COPY ./bin/etl /app/etl

EXPOSE 8088
