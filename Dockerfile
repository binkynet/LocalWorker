FROM scratch
ARG GOARCH=amd64

ADD bin/linux/${GOARCH}/bnLocalWorker /app/

ENTRYPOINT ["/app/bnLocalWorker"]
