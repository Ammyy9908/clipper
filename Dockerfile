FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -o /out/worker ./cmd/worker

FROM alpine:3.20
# yt-dlp from pip, not apk: the distro package lags badly and a stale yt-dlp is
# the single largest cause of outages in this service. Rebuild this image on a
# schedule (weekly at minimum) even when your own code hasn't changed.
RUN apk add --no-cache ffmpeg ca-certificates python3 py3-pip && \
    pip3 install --no-cache-dir --break-system-packages yt-dlp && \
    yt-dlp --version

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/worker /usr/local/bin/worker
ENV WORK_DIR=/scratch
RUN mkdir -p /scratch
EXPOSE 8080
CMD ["api"]
