#!/bin/bash

# Build the Docker image
docker build -t tellulf .
docker run -p 3000:3000 --env-file ./.env tellulf