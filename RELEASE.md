# Release process for WineVault

This project builds a single container image that contains both the Go API and the Nuxt frontend. The image is published to GitHub Container Registry (GHCR) and can be run with Docker Compose or Kubernetes.

## 1. Prepare the release

1. Ensure the working tree is clean and all tests pass locally.
2. Update the project version if needed in the Git tag you will publish.
3. Confirm the image target is correct in the workflow and the GHCR repository name matches your GitHub org/user.

Typical local validation:

```bash
git checkout main
cd backend
go test ./...
cd ../frontend
npm ci
npm run build
```

## 2. Create a git tag

Create a semantic version tag and push it to GitHub:

```bash
git tag v1.2.3
git push origin v1.2.3
```

The GitHub Actions workflow is configured to run on tag pushes and publish the image to GHCR. It also creates a `latest` tag from the default branch.

## 3. Trigger the GitHub Action

The workflow file is:

- .github/workflows/release-image.yml

It runs automatically when a tag matching `v*` is pushed, or when a GitHub release is published. It can also be run manually from the Actions tab with the "Run workflow" button.

## 4. Published image

The image is pushed to:

```text
ghcr.io/<your-github-user-or-org>/WineVault:tag
```

For example:

```text
ghcr.io/jLemmings/WineVault:v1.2.3
```

The workflow also produces `latest` for the default branch.

## 5. Run the published image with Docker Compose

Copy the example environment file and edit it for your registry and secrets:

```bash
cp docker-compose.prod.example.yml docker-compose.yml
```

Then set an image tag and run:

```bash
export WINEVAULT_TAG=v1.2.3
docker compose up -d
```

If you want to pin to a specific GHCR image:

```yaml
app:
  image: ghcr.io/jLemmings/WineVault:v1.2.3
```

The Compose stack starts PostgreSQL and the app together.

## 6. Deploy with Helm

From the repo root:

```bash
helm upgrade --install winevault ./helm/winevault \
  --set image.repository=ghcr.io/<your-github-user-or-org>/winevault \
  --set image.tag=v1.2.3
```

To expose the app via ingress:

```bash
helm upgrade --install winevault ./helm/winevault \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=winevault.example.com
```

## 7. Required environment settings for production

The application expects PostgreSQL to be reachable and uses the backend environment variables described in the project README. For production deployments, set at least:

- `DATABASE_URL`
- `HTTP_ADDR`
- `AUTH_COOKIE_SECURE=true` when behind HTTPS or a reverse proxy
- `OPENAI_API_KEY` if label recognition is enabled
- `GRAPEMINDS_API_KEY` if GrapeMinds lookup is enabled

## 8. Release checklist

- [ ] All local validation passes
- [ ] Tag created and pushed
- [ ] GitHub Actions workflow publishes the image
- [ ] Registry image is visible in GHCR
- [ ] Compose file points to the correct release tag
- [ ] Secrets are set in your runtime environment
- [ ] TLS or ingress is configured in front of the app

## 9. Notes

This is a single-user local application by default. Before public exposure, add proper authentication, TLS termination, and access controls in front of the service.
