FROM golang:1.27.2-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Regenerate the templ components (so the build never uses stale generated
# code), check, test, then build a static binary with all static files
# embedded.
RUN go generate ./... \
	&& go vet ./... \
	&& go test ./... \
	&& CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tellulf .

FROM gcr.io/distroless/static-debian12

COPY --from=build /tellulf /tellulf

EXPOSE 3000

ENTRYPOINT ["/tellulf"]
