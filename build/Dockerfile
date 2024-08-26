FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    libssl3 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY ./lib /usr/local/lib
COPY ./bin/skyeye /app/skyeye

ENV LD_LIBRARY_PATH=/usr/local/lib
EXPOSE 8088