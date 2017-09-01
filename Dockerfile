FROM scratch

ADD ./bnLocalWorker /app/

ENTRYPOINT ["/app/bnLocalWorker"]
