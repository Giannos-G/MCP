.PHONY: build up down logs ps clean test

build:
	docker-compose build --quiet

up:
	docker-compose up -d --build

down:
	docker-compose down

logs:
	docker-compose logs -f

ps:
	docker-compose ps

clean:
	docker-compose down -v --rmi local

test:
	docker-compose --profile test run --rm smoke-test
