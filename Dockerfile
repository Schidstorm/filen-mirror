FROM golang:1.26-alpine AS builder

WORKDIR /app
COPY . .
RUN go build -o filen-mirror ./main.go

FROM alpine:3.22

WORKDIR /app
COPY --from=builder /app/filen-mirror .

RUN addgroup -S -g 10001 app \
	&& adduser -S -D -H -u 10001 -G app app \
	&& mkdir /data \
	&& chown app:app /data
USER 10001


ENV TOTP_DIGITS=6
ENV TOTP_PERIOD=30
ENV FILEN_EMAIL=
ENV FILEN_PASSWORD=
ENV FILEN_TOTP_SECRET=
ENV FILEN_SOCKET_URL=wss://socket.filen.io:443
ENV FILEN_SYNC_DIR=/data
VOLUME /data

CMD ["./filen-mirror"]