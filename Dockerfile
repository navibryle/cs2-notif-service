FROM golang:1.22.2-bullseye

RUN mkdir -p /home
WORKDIR /home
RUN git clone https://github.com/navibryle/cs2-notif-service.git
WORKDIR /home/cs2-notif-service
CMD go run .
