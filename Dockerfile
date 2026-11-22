FROM golang:1.23-alpine AS build
WORKDIR /src
COPY singularity/go.mod ./
COPY singularity/cmd ./cmd
RUN CGO_ENABLED=0 go build -o /singularity ./cmd/singularity

FROM alpine:3.20
COPY --from=build /singularity /usr/local/bin/singularity
VOLUME /data/artifacts
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/singularity"]
