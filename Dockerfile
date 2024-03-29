FROM public.ecr.aws/docker/library/golang:alpine AS builder
RUN apk add git
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o rated

FROM scratch
EXPOSE 8080
COPY --from=builder /app/rated /
CMD ["/rated", "-addr=:8080"]
