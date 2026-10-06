FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/pocketful .

FROM scratch
COPY --from=build /out/pocketful /pocketful
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/pocketful"]
