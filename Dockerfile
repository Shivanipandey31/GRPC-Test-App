FROM alpine:3.20
RUN addgroup -S app && adduser -S app -G app
USER app
COPY build/server /server
EXPOSE 50051
ENTRYPOINT ["/server"]
