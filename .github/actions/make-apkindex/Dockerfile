
FROM alpine:edge
RUN apk --update add alpine-sdk openssl
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT /entrypoint.sh
