# Nazwa binarki
BINARY_NAME=sqoush-app

# Komendy główne
all: build

# 1. Generowanie kodu Go z plików .templ
templ:
	@if command -v templ >/dev/null; then templ generate; else echo "templ nie jest zainstalowany, pomijam generate"; fi

# 2. Budowanie CSS (minifikacja dla produkcji)
css:
	npx @tailwindcss/cli -i ./static/css/input.css -o ./static/css/output.css --minify

# 3. Budowanie CSS w trybie watch (do developmentu)
css-watch:
	npx @tailwindcss/cli -i ./static/css/input.css -o ./static/css/output.css --watch

# 4. Budowanie aplikacji (Templ -> CSS -> Go Build)
build: templ css
	go build -o ./bin/$(BINARY_NAME) ./main.go

# 5. Uruchomienie lokalne z Air (Hot Reload)
# Air musi być skonfigurowany, aby obserwować pliki .templ i .go
dev:
	make templ
	# Uruchamiamy css-watch w tle i air
	(trap 'kill 0' SIGINT; make css-watch & air)

# 6. Budowanie pod AWS Lambda (Linux ARM64 - taniej i szybciej na AWS Graviton)
build-lambda: templ css
	GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o bootstrap ./main.go
	zip lambda-handler.zip bootstrap

# Czyszczenie
clean:
	go clean
	rm -f ./bin/$(BINARY_NAME)
	rm -f bootstrap
	rm -f lambda-handler.zip
