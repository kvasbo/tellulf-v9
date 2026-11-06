# Holds only the binary: .github/workflows/build.yml generates, checks, tests
# and compiles it first, with Go's cache kept between runs. To build this by
# hand, make a Linux binary first:
#   CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o dist/tellulf .
FROM gcr.io/distroless/static-debian13

COPY dist/tellulf /tellulf

EXPOSE 3000

ENTRYPOINT ["/tellulf"]
