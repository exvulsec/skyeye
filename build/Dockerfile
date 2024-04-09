FROM debian:buster-slim

RUN apt update -y && apt install ca-certificates -y

WORKDIR /app

COPY ./bin/skyeye /app/skyeye

EXPOSE 8088
