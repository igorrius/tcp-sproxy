.PHONY: test-integration

test-integration:
	docker-compose -f integration_tests/docker-compose.yml up -d --build
	docker-compose -f integration_tests/docker-compose.yml run --rm test-runner || (docker-compose -f integration_tests/docker-compose.yml logs && docker-compose -f integration_tests/docker-compose.yml down && exit 1)
	docker-compose -f integration_tests/docker-compose.yml down
