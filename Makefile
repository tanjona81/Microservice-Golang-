# Variables
IMAGE_NAME = localhost:5000/my-go-app
TAG := $(shell git rev-parse --short HEAD) # Use the current git hash to ensure unique tags
FULL_IMAGE = $(IMAGE_NAME):$(TAG)
NAMESPACE=ingress-nginx
VALUES=infra/ingress-values.yaml
CHART=ingress-nginx/ingress-nginx

.PHONY: build push deploy clean ingress-init ingress-install ingress-status

# Used for deploying modification
dev: build push deploy
	@echo "New version $(TAG) is live!"

# Setup infrasctructure manually
setup-infra: ingress-init ingress-install
	@echo "Infrastructure is ready."

# Build the image
build:
	@echo "Building image $(FULL_IMAGE)..."
	docker build -t $(FULL_IMAGE) .

# Push to your local registry
push:
	@echo "Pushing $(FULL_IMAGE) to registry..."
	docker push $(FULL_IMAGE)

# Update the deployment and apply
deploy: build push
	@echo "Updating manifest with tag $(TAG)..."
	# This sed command finds the 'image:' line and replaces it with our new tag
	sed -i 's|image: .*|image: $(FULL_IMAGE)|' k8s/03-app.yaml
	@echo "Applying deployment..."
	kubectl apply -f k8s/03-app.yaml

# Clean up
clean:
	docker rmi $(FULL_IMAGE)

# Initialize the repo (Only needs to be done once)
ingress-init:
	helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
	helm repo update

# The "Atomic" Install
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

# Check status
ingress-status:
	kubectl get pods -n $(NAMESPACE) -o wide
	kubectl get ingress -A