# Variables
NAMESPACE=ingress-nginx
VALUES=infra/ingress-values.yaml
CHART=ingress-nginx/ingress-nginx

.PHONY: ingress-init ingress-install ingress-status

# 1. Initialize the repo (Only needs to be done once)
ingress-init:
	helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
	helm repo update

# 2. The "Atomic" Install
ingress-install:
	@echo "Installing Ingress Controller..."
	helm upgrade --install ingress-nginx $(CHART) \
		--namespace $(NAMESPACE) --create-namespace \
		-f $(VALUES)
	@echo "Waiting for Ingress to be ready (avoiding 503 errors)..."
	kubectl wait --namespace $(NAMESPACE) \
		--for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller \
		--timeout=120s

# 3. Check status
ingress-status:
	kubectl get pods -n $(NAMESPACE) -o wide
	kubectl get ingress -A

# # Variables
# IMAGE_NAME = localhost:5000/my-go-app
# # Use the current git hash to ensure unique tags
# TAG := $(shell git rev-parse --short HEAD)
# FULL_IMAGE = $(IMAGE_NAME):$(TAG)

# .PHONY: build push deploy clean

# # 1. Build the image
# build:
# 	@echo "Building image $(FULL_IMAGE)..."
# 	docker build -t $(FULL_IMAGE) .

# # 2. Push to your local registry
# push:
# 	@echo "Pushing $(FULL_IMAGE) to registry..."
# 	docker push $(FULL_IMAGE)

# # 3. Update the deployment and apply
# deploy: build push
# 	@echo "Updating manifest with tag $(TAG)..."
# 	# This sed command finds the 'image:' line and replaces it with our new tag
# 	sed -i 's|image: .*|image: $(FULL_IMAGE)|' k8s/base/deployment.yaml
# 	@echo "Applying deployment..."
# 	kubectl apply -f k8s/base/deployment.yaml

# # 4. Clean up
# clean:
# 	docker rmi $(FULL_IMAGE)