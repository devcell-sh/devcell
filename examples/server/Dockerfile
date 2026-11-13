# Example: extend a baked devcell image for production deployment.
#
# 1. Build the base image (bakes /nix into layers, no volume needed):
#      cell build --no-nix-volume
#
# 2. Build the server image on top:
#      docker build -t myapp-server .
#
# 3. Run without the devcell CLI or any named volumes:
#      docker run --rm -p 8080:8080 myapp-server

ARG BASE_IMAGE=localhost/devcell-user:base
FROM ${BASE_IMAGE}

COPY main.go /app/main.go
WORKDIR /app

RUN go build -o server main.go

EXPOSE 8080
CMD ["/app/server"]
