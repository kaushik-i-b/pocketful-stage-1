# Pocketful stage 1

Build and start the service with no manual setup:

```sh
docker build -t pocketful-stage1 . && docker run --rm -p 8080:8080 -e PORT=8080 pocketful-stage1
```

The process listens on `0.0.0.0` and the port in `PORT` (default `8080`). `GET /health` returns `{"status":"ok"}` when the service can accept requests.
