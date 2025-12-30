FROM golang AS builder

WORKDIR /app
COPY . .
RUN go build -o filen-mirror ./main.go

FROM golang

WORKDIR /app
COPY --from=builder /app/filen-mirror .

ENV TOTP_DIGITS=6
ENV TOTP_PERIOD=30
ENV FILEN_EMAIL=
ENV FILEN_PASSWORD=
ENV FILEN_TOTP_SECRET=
ENV FILEN_SOCKET_URL=wss://socket.filen.io:443
ENV FILEN_SYNC_DIR=/data
VOLUME /data

CMD ["./filen-mirror"]