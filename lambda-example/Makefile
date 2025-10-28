build:
	./build.sh

bash:
	docker compose run --rm app bash

deploy: build
	docker compose run --rm app cdk deploy --require-approval never
