FROM alpine
RUN apk --no-cache add brotli tar
COPY extractor.sh /bin/extractor
ENTRYPOINT ["/bin/extractor"]
