Jak uruchomić lokalnie

1. Zainstaluj zależności JS (jeśli chcesz Tailwind/Daisy):

npm install

2. Wygeneruj CSS (opcjonalne, ale zalecane):

make css

3. Uruchom serwer:

go run ./main.go

Aplikacja będzie na http://localhost:8080.

Uwaga o Go modules

- Nie mogłem uruchomić go mod tidy przez brak dostępu do internetu w sandboxie.
- U Ciebie uruchom:

GOCACHE=/tmp/go-cache go mod tidy

lub po prostu go mod tidy z dostępem do sieci.

Przygotowanie pod AWS Lambda + DynamoDB

- Klucz: masz już interfejs Store. Wystarczy dopisać DynamoStore implementujący ten interfejs.
- W main.go możesz dodać przełącznik, np. STORAGE=dynamo i podmienić implementację.
- Makefile już ma target build-lambda.
- Na przyszłość: dodamy internal/store/dynamo.go + konfigurację (region, tabela, etc.), a reszta kodu (handlers, views) zostaje bez zmian.

Co jeszcze warto dorobić jako następny krok

1. Zastąpić static/htmx.min.js prawdziwym HTMX.
2. Dodać prawdziwe logowanie (na razie jest mock przez cookie).
3. Dodać DynamoStore i przełącznik STORAGE.

Jeśli chcesz, mogę od razu:

1. Podpiąć prawdziwe HTMX.
2. Dodać DynamoStore (z konfiguracją tabel i kluczami).
3. Przerobić logowanie na OAuth (Google/Apple).

Powiedz tylko, od czego zaczynamy.


`go run ./main.go`
