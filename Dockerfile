FROM scratch
ARG GOARCH=amd64

ADD bin/${GOARCH}/bnLocalWorker /app/

ENTRYPOINT ["/app/bnLocalWorker"]
