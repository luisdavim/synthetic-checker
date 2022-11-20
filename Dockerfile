FROM golang:1.19-alpine as builder

# deinitializing GOPATH as otherwise go modules don't work properly
ENV GOPATH=""

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY pkg ./pkg
COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /synthetic-checker -trimpath -ldflags="-s -w -extldflags '-static'"

FROM alpine:3.15

COPY --from=builder /synthetic-checker /
ENTRYPOINT [ "/synthetic-checker" ]
