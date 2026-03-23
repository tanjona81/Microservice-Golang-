#!/bin/bash
# A simple "Deploy" script
TAG=$(git rev-parse --short HEAD) # Get current git hash
IMG="localhost:5000/my-go-app:$TAG"

docker build -t $IMG .
docker push $IMG
# Use 'sed' to update the image tag in your YAML automatically
sed -i "s|image:.*|image: $IMG|" k8s/base/deployment.yaml
kubectl apply -f k8s/base/deployment.yaml