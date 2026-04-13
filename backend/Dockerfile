FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server ./server
EXPOSE 8080
CMD ["./server"]
